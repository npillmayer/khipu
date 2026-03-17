package khipu

import (
	"bufio"
	"errors"

	"github.com/npillmayer/cords/styled"
)

// DiscretionaryCandidate stores one lazily discovered hyphenation alternative
// associated with a textbox knot. Variants are expected to be stable for a given
// textbox within one paragraph instance.
type DiscretionaryCandidate struct {
	Variant   uint16
	PreBreak  KnotCore
	PostBreak KnotCore
}

// DiscretionarySelection records which discretionary candidate was selected
// for a rendered break decision.
type DiscretionarySelection struct {
	Source  int
	Variant uint16
}

func defaultFlagsForKind(kind KnotType) KnotFlags {
	switch kind {
	case KTGlue, KTKern:
		return KFDiscardable
	default:
		return 0
	}
}

func (khipu *Khipu) flagsAt(index int) KnotFlags {
	if khipu == nil || index < 0 || index >= len(khipu.Flags) {
		return 0
	}
	return khipu.Flags[index]
}

func (k KnotCore) IsDiscardable() bool {
	return k.Flags&KFDiscardable != 0
}

func (khipu *Khipu) SetKnotFlags(index int, flags KnotFlags) bool {
	if khipu == nil || index < 0 || index >= len(khipu.Kind) {
		return false
	}
	if len(khipu.Flags) < len(khipu.Kind) {
		missing := len(khipu.Kind) - len(khipu.Flags)
		khipu.Flags = append(khipu.Flags, make([]KnotFlags, missing)...)
	}
	khipu.Flags[index] = flags
	return true
}

func (khipu *Khipu) AddDiscretionaryCandidate(index int, cand DiscretionaryCandidate) bool {
	if khipu == nil || index < 0 || index >= len(khipu.Kind) {
		return false
	}
	if khipu.DiscretionaryCandidates == nil {
		khipu.DiscretionaryCandidates = make(map[int][]DiscretionaryCandidate)
	}
	khipu.DiscretionaryCandidates[index] = append(khipu.DiscretionaryCandidates[index], cand)
	return true
}

func (khipu *Khipu) DiscretionariesAt(index int) []DiscretionaryCandidate {
	if khipu == nil || index < 0 {
		return nil
	}
	return khipu.DiscretionaryCandidates[index]
}

func (khipu *Khipu) SelectDiscretionary(breakpoint int, choice DiscretionarySelection) bool {
	if khipu == nil || breakpoint < 0 || breakpoint >= len(khipu.Kind) {
		return false
	}
	if khipu.SelectedDiscretionaries == nil {
		khipu.SelectedDiscretionaries = make(map[int]DiscretionarySelection)
	}
	khipu.SelectedDiscretionaries[breakpoint] = choice
	return true
}

func (khipu *Khipu) SelectedDiscretionaryAt(breakpoint int) (DiscretionarySelection, bool) {
	if khipu == nil || breakpoint < 0 {
		return DiscretionarySelection{}, false
	}
	choice, ok := khipu.SelectedDiscretionaries[breakpoint]
	return choice, ok
}

// DiscretionaryCandidates returns the currently known discretionary candidates
// for one textbox knot. If no candidates are cached yet, Khipukamayuq will
// consult its Hyphenator, shape the returned fragments, and cache the resulting
// discretionary candidates on the Khipu.
func (kq *Khipukamayuq) DiscretionaryCandidates(khp *Khipu, at int) ([]DiscretionaryCandidate, error) {
	if kq == nil {
		return nil, ErrVoidKhipukamayuq
	}
	if khp == nil || at < 0 || at >= len(khp.Kind) {
		return nil, ErrIllegalArguments
	}
	if khp.owner != nil && khp.owner != kq {
		return nil, ErrIllegalArguments
	}
	if cands := khp.DiscretionariesAt(at); len(cands) > 0 {
		return cands, nil
	}
	if kq.env.hyphenator == nil {
		return nil, nil
	}
	if khp.Kind[at] != KTTextBox {
		return nil, nil
	}
	word, pos, err := textboxSource(khp, at)
	if err != nil {
		return nil, err
	}
	styleChange, _ := styleChangeForKnot(khp, at)
	fragments, err := kq.env.hyphenator.Discretionaries(word, kq.env.params)
	if err != nil {
		return nil, err
	}
	for i, frag := range fragments {
		if frag.PreBreak == "" || frag.PostBreak == "" {
			continue
		}
		pre, err := kq.shapeDiscretionaryFragment(frag.PreBreak, pos, styleChange)
		if err != nil {
			return nil, err
		}
		postPos := pos + uint64(len(frag.PreBreak))
		post, err := kq.shapeDiscretionaryFragment(frag.PostBreak, postPos, styleChange)
		if err != nil {
			return nil, err
		}
		variant := frag.Variant
		if variant == 0 {
			variant = uint16(i + 1)
		}
		pre.Penalty = discretionaryPenalty(frag, kq.env.params)
		khp.AddDiscretionaryCandidate(at, DiscretionaryCandidate{
			Variant:   variant,
			PreBreak:  pre,
			PostBreak: post,
		})
	}
	return khp.DiscretionariesAt(at), nil
}

// discretionaryPenalty resolves the effective penalty of a discretionary
// fragment. A fragment may carry a script-specific penalty supplied by the
// Hyphenator; otherwise we fall back to Khipukamayuq's current parameter set.
func discretionaryPenalty(frag HyphenationFragment, params *Params) Penalty {
	if frag.Penalty != 0 {
		return frag.Penalty
	}
	if params == nil {
		return 0
	}
	return Penalty(params.Hyphenpenalty)
}

// textboxSource recovers the original textbox text and absolute paragraph
// position from an already encoded Khipu. This is the anchor for lazy
// hyphenation: the linebreaker only knows the knot index, while the Hyphenator
// and Shaper still need the source text.
func textboxSource(khp *Khipu, at int) (string, uint64, error) {
	if khp == nil || khp.paragraph == nil || at < 0 || at >= len(khp.Kind) {
		return "", 0, ErrIllegalArguments
	}
	raw := khp.paragraph.Raw().String()
	start := int(khp.Pos[at] - khp.paragraph.Offset)
	end := start + int(khp.Len[at])
	if start < 0 || end < start || end > len(raw) {
		return "", 0, errors.New("khipu: textbox source out of bounds")
	}
	return raw[start:end], khp.Pos[at], nil
}

// styleChangeForKnot finds the style run containing one textbox knot.
// Discretionary fragments are re-shaped under the same style as the original
// textbox so their width metrics stay consistent with the paragraph.
func styleChangeForKnot(khp *Khipu, at int) (styled.BidiStyleChange, bool) {
	if khp == nil || khp.paragraph == nil || at < 0 || at >= len(khp.Kind) {
		return styled.BidiStyleChange{}, false
	}
	start := uint64(khp.Pos[at] - khp.paragraph.Offset)
	end := start + uint64(khp.Len[at])
	for styleChange, _ := range khp.paragraph.BidiStyleRanges() {
		s0 := uint64(styleChange.Position)
		s1 := uint64(styleChange.Position + styleChange.Length)
		if start >= s0 && end <= s1 {
			return styleChange, true
		}
	}
	return styled.BidiStyleChange{}, false
}

// shapeDiscretionaryFragment turns one fragment string into a textbox-like
// KnotCore carrying measured width information. The resulting KnotCore does not
// yet decide linebreaking policy; the caller attaches the discretionary penalty
// afterwards.
func (kq *Khipukamayuq) shapeDiscretionaryFragment(text string, pos uint64, styleChange styled.BidiStyleChange) (KnotCore, error) {
	var br *bufio.Reader
	buf := []byte(text)
	br = wrapAsRuneReader(br, buf)
	kq.env.shaper.SetSource(br)
	kq.env.shaper.SetStyle(styleChange)
	metrics, err := getMetrics(len(buf), kq.env.shaper)
	if err != nil {
		return KnotCore{}, err
	}
	w := sumMetrics(metrics, 0, len(buf))
	k := KnotCore{
		Pos:     pos,
		Len:     uint16(len(buf)),
		W:       w,
		MinW:    w,
		MaxW:    w,
		Kind:    KTTextBox,
		Flags:   defaultFlagsForKind(KTTextBox),
		Penalty: 0,
	}
	return k, nil
}
