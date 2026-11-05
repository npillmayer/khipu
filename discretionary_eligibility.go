package khipu

// automaticHyphenationEligible reports whether an ordinary streaming textbox
// can safely be submitted as one complete source segment to a hyphenator. It
// inspects retained metadata only: no text copies, neighbor concatenation,
// collaborator calls, candidate discovery or cache changes occur here.
//
// A structural/run boundary is not a word boundary. Both source endpoints must
// be default UAX#29 word boundaries (paragraph start/end count), and no internal
// word boundary may be hidden inside a shaping cluster. Thus a word split by
// style or direction is declined on BOTH sides, even with equal font settings.
// Source gaps likewise cannot be repaired by borrowing neighboring text.
//
// True is only structural eligibility, not proof of a linguistic word or a
// useful hyphenation. Default segmentation includes numbers and punctuation and
// is not dictionary segmentation for every script. The language-aware policy
// must still decide whether to return alternatives. No letter/whitespace-based
// word heuristic or Western minimum length is imposed here. Soft-hyphen boxes
// use explicit discovery exclusively; automatic/explicit variants are not mixed.
//
// The streaming provider applies this before querying automatic alternatives.
// Encoded source, metadata and contexts must remain stable while caches exist.
func (k *Khipu) automaticHyphenationEligible(at int) bool {
	if k == nil || k.paragraph == nil || at < 0 || at >= len(k.Kind) ||
		k.Kind[at] != KTTextBox || k.flagsAt(at)&KFDiscardable != 0 ||
		at >= len(k.boxRefs) || at >= len(k.shaping.sources) {
		return false
	}
	id := k.boxRefs[at]
	if id <= 0 || int(id) > len(k.shaping.boxes) {
		return false // Includes legacy metric-only paragraphs.
	}
	box := k.shaping.boxes[id-1]
	if box.replacement != nil || box.context < 0 || box.context >= len(k.shaping.contexts) {
		return false
	}
	s := box.source
	bounds := k.shaping.boundaries
	if s != k.shaping.sources[at].source || s.scalarStart >= s.scalarEnd ||
		s.scalarEnd > uint64(len(bounds)) || s.byteStart >= s.byteEnd {
		return false
	}
	// Check byte/scalar agreement using the original paragraph table, not
	// breakAfter: an ordinary line break can be attached beyond a box's source.
	byteStart := uint64(0)
	if s.scalarStart > 0 {
		byteStart = bounds[s.scalarStart-1].byteEnd
		if !bounds[s.scalarStart-1].wordBoundary {
			return false
		}
	}
	if s.byteStart != byteStart || s.byteEnd != bounds[s.scalarEnd-1].byteEnd ||
		(s.scalarEnd < uint64(len(bounds)) && !bounds[s.scalarEnd-1].wordBoundary) {
		return false
	}
	run := k.shaping.contexts[box.context].source
	if s.byteStart < run.byteStart || s.byteEnd > run.byteEnd ||
		s.scalarStart < run.scalarStart || s.scalarEnd > run.scalarEnd {
		return false
	}
	for i := s.scalarStart; i < s.scalarEnd; i++ {
		if bounds[i].softHyphen || (i+1 < s.scalarEnd && bounds[i].wordBoundary) {
			return false
		}
	}
	return true
}
