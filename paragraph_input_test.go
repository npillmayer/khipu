package khipu

import (
	"context"
	"errors"
	"io"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
)

type atomicRecordingShaper struct {
	contracts.StreamingShaper
	texts  []string
	params []contracts.ShapeParams
	bases  []uint32
}

func (s *atomicRecordingShaper) SetInitialCluster(base uint32) error {
	s.bases = append(s.bases, base)
	return s.StreamingShaper.SetInitialCluster(base)
}
func (s *atomicRecordingShaper) Shape(ctx context.Context, p contracts.ShapeParams, r contracts.RuneReader, o contracts.ShapeBufferOptions) error {
	var text strings.Builder
	for {
		c, _, err := r.ReadRune()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		text.WriteRune(c)
	}
	s.texts = append(s.texts, text.String())
	p.Features = slices.Clone(p.Features)
	s.params = append(s.params, p)
	// The fixture records feature domains; the synthetic backend itself has
	// no feature implementation. These tests exercise run transport, not OT.
	p.Features = nil
	return s.StreamingShaper.Shape(ctx, p, strings.NewReader(text.String()), o)
}

func TestAtomicShapingBoundariesAndFeatures(t *testing.T) {
	var stream *atomicRecordingShaper
	defaults := preparationDefaults()
	defaults.Features = []contracts.ShapeFeature{{Start: 1, End: 4, Enabled: true}}
	resolves := 0
	owner := NewKhipukamayuq(Config{Streaming: &StreamingConfig{
		Defaults: defaults,
		Resolver: preparationResolver(func(_ styled.Style, d contracts.ResolvedRunStyle) (contracts.ResolvedRunStyle, error) {
			resolves++
			return d, nil
		}),
		NewShaper: func() (contracts.StreamingShaper, error) {
			s, err := helpers.NewSyntheticStreamingShaper(0)
			stream = &atomicRecordingShaper{StreamingShaper: s}
			return stream, err
		},
	}})
	anchors := []contracts.AtomicAnchor{{ID: 7, ByteOffset: 4}}
	boundaries := []AtomicBoundary{{ByteOffset: 4, Action: ProhibitBoundary}, {ByteOffset: 7, Action: AllowBoundary}}
	para := preparationParagraph(t, "éβ\uFFFCcd")
	para.Offset = 37
	p, err := owner.PrepareParagraph(t.Context(), ParagraphInput{para, anchors}, PrepareOptions{AtomicBoundaries: boundaries})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(stream.texts, []string{"éβ", "cd"}) || !slices.Equal(stream.bases, []uint32{0, 3}) || resolves != 1 {
		t.Fatal(stream.texts, stream.bases, resolves)
	}
	for i, want := range [][2]int{{1, 2}, {0, 1}} {
		f := stream.params[i].Features
		if len(f) != 1 || f[0].Start != want[0] || f[0].End != want[1] {
			t.Fatal("feature domain moved", f)
		}
	}
	anchors[0].ID = 99
	boundaries[1].Action = ProhibitBoundary
	k, err := owner.BindParagraph(t.Context(), p, []AtomicAdvance{{7, 15}})
	if err != nil {
		t.Fatal(err)
	}
	if len(k.Kind) != 4 || k.Kind[1] != KTAtomic || k.Pos[1] != 41 || k.Len[1] != 3 || k.Penalty[0] != MaxPenalty || k.Penalty[1] != 0 || k.boxRefs[1] != 0 {
		t.Fatal("mixed knots", k)
	}
	if k.shaping.sources[2].source.scalarStart != 3 || k.shaping.glyphs[2].Cluster != 3 {
		t.Fatal("lost original scalar coordinates")
	}
	resolves = 0
	_, err = owner.PrepareParagraph(t.Context(), ParagraphInput{preparationParagraph(t, "\uFFFC"), []contracts.AtomicAnchor{{ID: 1}}}, PrepareOptions{})
	if err != nil || resolves != 0 || len(stream.texts) != 0 {
		t.Fatal("object-only input resolved or shaped", err, resolves, stream.texts)
	}
	_, err = owner.PrepareParagraph(t.Context(), ParagraphInput{Paragraph: preparationParagraph(t, "\uFFFC")}, PrepareOptions{})
	if err != nil || !slices.Equal(stream.texts, []string{"\uFFFC"}) {
		t.Fatal("unattached marker is not ordinary text", err, stream.texts)
	}
}

func TestAtomicInputValidation(t *testing.T) {
	owner := NewKhipukamayuq(Config{})
	para := preparationParagraph(t, "é\uFFFC\uFFFCx")
	valid := []contracts.AtomicAnchor{{ID: 1, ByteOffset: 2}, {ID: 2, ByteOffset: 5}}
	for _, anchors := range [][]contracts.AtomicAnchor{
		{{ID: 0, ByteOffset: 2}}, {{ID: 1, ByteOffset: 2}, {ID: 1, ByteOffset: 5}},
		{{ID: 1, ByteOffset: 5}, {ID: 2, ByteOffset: 2}}, {{ID: 1, ByteOffset: 2}, {ID: 2, ByteOffset: 2}},
		{{ID: 1, ByteOffset: 0}}, {{ID: 1, ByteOffset: 1}}, {{ID: 1, ByteOffset: 3}},
		{{ID: 1, ByteOffset: 8}}, {{ID: 1, ByteOffset: 9}}, {{ID: 1, ByteOffset: ^uint64(0)}},
	} {
		if p, err := owner.PrepareParagraph(t.Context(), ParagraphInput{para, anchors}, PrepareOptions{}); p != nil || !errors.Is(err, ErrIllegalArguments) {
			t.Fatalf("anchors %v: %v %v", anchors, p, err)
		}
	}
	for _, bounds := range [][]AtomicBoundary{
		{{0, AllowBoundary}}, {{9, AllowBoundary}}, {{3, AllowBoundary}}, {{2, 0}}, {{2, 99}}, {{5, AllowBoundary}, {2, AllowBoundary}}, {{5, AllowBoundary}, {5, ProhibitBoundary}},
	} {
		if p, err := owner.PrepareParagraph(t.Context(), ParagraphInput{para, valid}, PrepareOptions{AtomicBoundaries: bounds}); p != nil || !errors.Is(err, ErrIllegalArguments) {
			t.Fatalf("boundaries %v: %v %v", bounds, p, err)
		}
	}
	mandatory := ParagraphInput{preparationParagraph(t, "a\n\uFFFC"), []contracts.AtomicAnchor{{ID: 1, ByteOffset: 2}}}
	for _, action := range []BoundaryAction{AllowBoundary, ProhibitBoundary} {
		if p, err := owner.PrepareParagraph(t.Context(), mandatory, PrepareOptions{AtomicBoundaries: []AtomicBoundary{{2, action}}}); p != nil || !errors.Is(err, ErrUnsupportedPolicy) {
			t.Fatal(p, err)
		}
	}
	for _, mode := range []HyphenationMode{ExplicitHyphenation, AutomaticHyphenation, 99} {
		if p, err := owner.PrepareParagraph(t.Context(), ParagraphInput{para, valid}, PrepareOptions{Hyphenation: mode}); p != nil || !errors.Is(err, ErrIllegalArguments) {
			t.Fatal(p, err)
		}
	}
	p, err := owner.PrepareParagraph(t.Context(), ParagraphInput{para, valid}, PrepareOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, advances := range [][]AtomicAdvance{
		nil, {{1, 1}}, {{1, 1}, {1, 2}}, {{1, 1}, {3, 2}}, {{0, 1}, {2, 2}}, {{1, dimen.Fil}, {2, 2}}, {{1, -dimen.Fil}, {2, 2}}, {{1, dimen.Infinity}, {2, 2}},
	} {
		if k, err := owner.BindParagraph(t.Context(), p, advances); k != nil || !errors.Is(err, ErrIllegalArguments) {
			t.Fatal(advances, k, err)
		}
	}
	if k, err := NewKhipukamayuq(Config{}).BindParagraph(t.Context(), p, []AtomicAdvance{{1, 1}, {2, 2}}); k != nil || !errors.Is(err, ErrIllegalArguments) {
		t.Fatal(k, err)
	}
	for _, p := range []*PreparedParagraph{nil, {}} {
		if k, err := owner.BindParagraph(t.Context(), p, nil); k != nil || !errors.Is(err, ErrIllegalArguments) {
			t.Fatal(k, err)
		}
	}
}

func TestAtomicBindingOwnershipAndHyphenation(t *testing.T) {
	for _, mode := range []HyphenationMode{NoHyphenation, ExplicitHyphenation, AutomaticHyphenation} {
		policy := &testhyphenator{want: []HyphenationFragment{{PreBreak: "-"}}}
		streams := 0
		owner := NewKhipukamayuq(Config{Hyphenator: policy, Streaming: &StreamingConfig{
			Resolver: helpers.DefaultStyleResolver{}, Defaults: preparationDefaults(),
			NewShaper: func() (contracts.StreamingShaper, error) { streams++; return helpers.NewSyntheticStreamingShaper(0) },
		}})
		p, err := owner.PrepareParagraph(t.Context(), ParagraphInput{preparationParagraph(t, "ab\u00adcd\uFFFC"), []contracts.AtomicAnchor{{ID: 1, ByteOffset: 6}}}, PrepareOptions{Hyphenation: mode})
		if err != nil {
			t.Fatal(err)
		}
		advances := []AtomicAdvance{{1, 10}}
		a, err := owner.BindParagraph(t.Context(), p, advances)
		if err != nil {
			t.Fatal(err)
		}
		advances[0].Advance = 20
		b, err := owner.BindParagraph(t.Context(), p, advances)
		if err != nil {
			t.Fatal(err)
		}
		if streams != 1 || a.W[1] != 10 || b.W[1] != 20 {
			t.Fatal("bind reshaped or aliased dimensions")
		}
		baseGlyphs := slices.Clone(p.shaping.glyphs)
		cands, err := owner.DiscretionaryCandidates(a, 0)
		if err != nil {
			t.Fatal(err)
		}
		if (len(cands) > 0) != (mode != NoHyphenation) {
			t.Fatal("wrong policy", mode, cands)
		}
		if len(cands) > 0 {
			a.SelectedDiscretionaries[0] = DiscretionarySelection{Source: 0, Variant: cands[0].Variant}
		}
		a.W[0]++
		a.Penalty[0] = 0
		a.Flags[0] = KFDiscardable
		a.boxRefs[0] = 0
		if b.W[0] != p.knots[0].W || b.Penalty[0] != p.knots[0].Penalty || b.Flags[0] != p.knots[0].Flags || b.boxRefs[0] != p.boxRefs[0] || len(b.SelectedDiscretionaries) != 0 || len(b.DiscretionaryCandidates) != 0 || !reflect.DeepEqual(baseGlyphs, b.shaping.glyphs) {
			t.Fatal("binding mutated sibling")
		}
		if !reflect.DeepEqual(baseGlyphs, p.shaping.glyphs) || len(p.shaping.boxes) != len(b.shaping.boxes) {
			t.Fatal("prepared arena changed")
		}
		calls := policy.calls
		if c, err := owner.DiscretionaryCandidates(b, 1); err != nil || len(c) != 0 || calls != policy.calls {
			t.Fatal("atom reached hyphenator", c, err)
		}
		// A fresh binding after arena growth retains precisely the initial state.
		c, err := owner.BindParagraph(t.Context(), p, advances)
		if err != nil || !reflect.DeepEqual(c.shaping, b.shaping) || len(c.DiscretionaryCandidates) != 0 {
			t.Fatal("later binding changed", err)
		}
	}
}

func TestAtomicPreparationFailureAndCancellation(t *testing.T) {
	failure := errors.New("backend failure")
	fail := true
	owner := NewKhipukamayuq(Config{Streaming: &StreamingConfig{Resolver: helpers.DefaultStyleResolver{}, Defaults: preparationDefaults(), NewShaper: func() (contracts.StreamingShaper, error) {
		if fail {
			return nil, failure
		}
		return helpers.NewSyntheticStreamingShaper(0)
	}}})
	in := ParagraphInput{preparationParagraph(t, "x\uFFFC"), []contracts.AtomicAnchor{{ID: 1, ByteOffset: 1}}}
	if p, err := owner.PrepareParagraph(t.Context(), in, PrepareOptions{}); p != nil || !errors.Is(err, failure) {
		t.Fatal(p, err)
	}
	fail = false
	p, err := owner.PrepareParagraph(t.Context(), in, PrepareOptions{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if p, err := (*Khipukamayuq)(nil).PrepareParagraph(ctx, ParagraphInput{}, PrepareOptions{}); p != nil || !errors.Is(err, context.Canceled) {
		t.Fatal(p, err)
	}
	if k, err := (*Khipukamayuq)(nil).BindParagraph(ctx, nil, nil); k != nil || !errors.Is(err, context.Canceled) {
		t.Fatal(k, err)
	}
	if p, err := owner.PrepareParagraph(nil, in, PrepareOptions{}); p != nil || !errors.Is(err, ErrIllegalArguments) {
		t.Fatal(p, err)
	}
	if k, err := owner.BindParagraph(nil, p, nil); k != nil || !errors.Is(err, ErrIllegalArguments) {
		t.Fatal(k, err)
	}
	if k, err := owner.BindParagraph(t.Context(), p, []AtomicAdvance{{1, 10}}); err != nil || k == nil {
		t.Fatal(k, err)
	}
}

func TestAtomicHyphenationDiscoveryModes(t *testing.T) {
	for _, mode := range []HyphenationMode{NoHyphenation, ExplicitHyphenation, AutomaticHyphenation} {
		var words []string
		owner := automaticTestOwner(explicitMaterialFunc(func(word string, _ *Params) ([]HyphenationFragment, error) {
			words = append(words, word)
			return nil, nil
		}), nil)
		p, err := owner.PrepareParagraph(t.Context(), ParagraphInput{
			Paragraph: preparationParagraph(t, "abcd\uFFFCefgh"),
			Atomics:   []contracts.AtomicAnchor{{ID: 1, ByteOffset: 4}},
		}, PrepareOptions{Hyphenation: mode})
		if err != nil {
			t.Fatal(err)
		}
		k, err := owner.BindParagraph(t.Context(), p, []AtomicAdvance{{ID: 1, Advance: 10}})
		if err != nil {
			t.Fatal(err)
		}
		for i := range k.Kind {
			if _, err := owner.DiscretionaryCandidates(k, i); err != nil {
				t.Fatal(err)
			}
		}
		if mode == AutomaticHyphenation {
			if !slices.Equal(words, []string{"abcd", "efgh"}) {
				t.Fatal("wrong automatic segments", words)
			}
		} else if len(words) != 0 {
			t.Fatal("automatic discovery in disabled mode", mode, words)
		}
	}
}

func TestAtomicPreparationStreamFailurePublishesNothing(t *testing.T) {
	for _, mode := range []string{"consumer", "terminal"} {
		s := newPipelineShaper()
		want := error(ErrIllegalArguments)
		if mode == "consumer" {
			s.badAnchor = true
			s.wait = true
		} else {
			want = errors.New("terminal stream failure")
			s.terminalErr = want
		}
		owner := NewKhipukamayuq(Config{Streaming: &StreamingConfig{Resolver: helpers.DefaultStyleResolver{}, Defaults: preparationDefaults(), NewShaper: func() (contracts.StreamingShaper, error) { return s, nil }}})
		in := ParagraphInput{Paragraph: preparationParagraph(t, "\uFFFCa"), Atomics: []contracts.AtomicAnchor{{ID: 1}}}
		if p, err := owner.PrepareParagraph(t.Context(), in, PrepareOptions{}); p != nil || !errors.Is(err, want) {
			t.Fatal(p, err)
		}
		select {
		case <-s.exited:
		default:
			t.Fatal("producer not joined")
		}
		owner.env.streaming.NewShaper = func() (contracts.StreamingShaper, error) { return helpers.NewSyntheticStreamingShaper(0) }
		if p, err := owner.PrepareParagraph(t.Context(), in, PrepareOptions{}); p == nil || err != nil {
			t.Fatal("retry failed", p, err)
		}
	}
}
