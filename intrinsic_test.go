package khipu

import (
	"context"
	"errors"
	"math"
	"reflect"
	"slices"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
	"github.com/npillmayer/uax/bidi"
)

func intrinsicOwner() *Khipukamayuq {
	return NewKhipukamayuq(Config{Streaming: &StreamingConfig{
		Resolver:  helpers.DefaultStyleResolver{},
		Defaults:  contracts.ResolvedRunStyle{Font: helpers.NewSyntheticFont(), EmSize: 10 * dimen.BP, Spacing: contracts.SpacingElastic},
		NewShaper: func() (contracts.StreamingShaper, error) { return helpers.NewSyntheticStreamingShaper(8) },
	}})
}

func intrinsicPrepare(t *testing.T, owner *Khipukamayuq, text string, options PrepareOptions) *PreparedParagraph {
	t.Helper()
	var anchors []contracts.AtomicAnchor
	for offset, r := range text {
		if r == '\uFFFC' {
			anchors = append(anchors, contracts.AtomicAnchor{ID: contracts.AtomicID(len(anchors) + 1), ByteOffset: uint64(offset)})
		}
	}
	p, err := owner.PrepareParagraph(t.Context(), ParagraphInput{preparationParagraph(t, text), anchors}, options)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestIntrinsicOrdinaryWidths(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		atoms      []AtomicIntrinsic
		boundaries []AtomicBoundary
		want       IntrinsicWidths
	}{
		{"empty", "", nil, nil, IntrinsicWidths{}},
		{"whitespace", "   ", nil, nil, IntrinsicWidths{}},
		{"words", "alpha beta", nil, nil, IntrinsicWidths{25 * dimen.BP, 50 * dimen.BP}},
		{"trim", " ab  c ", nil, nil, IntrinsicWidths{10 * dimen.BP, 25 * dimen.BP}},
		{"nbsp interior", "ab\u00a0cd", nil, nil, IntrinsicWidths{25 * dimen.BP, 25 * dimen.BP}},
		{"soft hyphen disabled", "ab\u00adcd", nil, nil, IntrinsicWidths{20 * dimen.BP, 20 * dimen.BP}},
		{"multibyte bidi", "éβ אב", nil, nil, IntrinsicWidths{10 * dimen.BP, 25 * dimen.BP}},
		{"atomic range", "\uFFFC", []AtomicIntrinsic{{1, 10, 30}}, nil, IntrinsicWidths{10, 30}},
		{"adjacent atom ranges", "\uFFFC\uFFFC", []AtomicIntrinsic{{2, 30, 40}, {1, 10, 20}}, nil, IntrinsicWidths{30, 60}},
		{"prohibited", "\uFFFC\uFFFC", []AtomicIntrinsic{{1, 10, 20}, {2, 30, 40}}, []AtomicBoundary{{3, ProhibitBoundary}}, IntrinsicWidths{40, 60}},
		{"allowed", "\uFFFC\uFFFC", []AtomicIntrinsic{{1, 10, 20}, {2, 30, 40}}, []AtomicBoundary{{3, AllowBoundary}}, IntrinsicWidths{30, 60}},
		{"zero shelters interior space", "\uFFFC ab ", []AtomicIntrinsic{{1, 0, 0}}, nil, IntrinsicWidths{10 * dimen.BP, 15 * dimen.BP}},
		{"zero end shelters space", " ab \uFFFC", []AtomicIntrinsic{{1, 0, 0}}, nil, IntrinsicWidths{10 * dimen.BP, 15 * dimen.BP}},
		{"forced boundary", "\uFFFC\n\uFFFC", []AtomicIntrinsic{{1, 10, 20}, {2, 30, 40}}, nil, IntrinsicWidths{30, 40}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			owner := intrinsicOwner()
			p := intrinsicPrepare(t, owner, tc.text, PrepareOptions{AtomicBoundaries: tc.boundaries})
			got, err := owner.MeasureIntrinsic(t.Context(), p, MeasureRequest{Atomics: tc.atoms})
			if err != nil || got != tc.want {
				t.Fatalf("got %+v, %v; want %+v", got, err, tc.want)
			}
		})
	}
}

type intrinsicStyle string

func (s intrinsicStyle) Equals(other styled.Style) bool { return s == other }
func (s intrinsicStyle) String() string                 { return string(s) }

func TestIntrinsicUnbreakableStyledWord(t *testing.T) {
	owner := intrinsicOwner()
	owner.env.streaming.Resolver = preparationResolver(func(_ styled.Style, d contracts.ResolvedRunStyle) (contracts.ResolvedRunStyle, error) { return d, nil })
	text := styled.TextFromString("abcd")
	var err error
	text, err = text.Style(intrinsicStyle("first"), 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	text, err = text.Style(intrinsicStyle("second"), 2, 4)
	if err != nil {
		t.Fatal(err)
	}
	para, err := styled.ParagraphFromText(&text, 0, 4, bidi.LeftToRight, nil)
	if err != nil {
		t.Fatal(err)
	}
	p, err := owner.PrepareParagraph(t.Context(), ParagraphInput{Paragraph: para}, PrepareOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.knots) != 3 {
		t.Fatal("fixture must have two text knots and fill", p.knots)
	}
	got, err := owner.MeasureIntrinsic(t.Context(), p, MeasureRequest{})
	if err != nil || got != (IntrinsicWidths{20 * dimen.BP, 20 * dimen.BP}) {
		t.Fatal("style boundary became a wrap", got, err)
	}
}

func TestIntrinsicSignedDomain(t *testing.T) {
	for _, tc := range []struct {
		name   string
		values []AtomicIntrinsic
		join   bool
		want   IntrinsicWidths
		bad    bool
	}{
		{"negative span", []AtomicIntrinsic{{1, -30, -20}}, false, IntrinsicWidths{}, false},
		{"negative atom not clamped", []AtomicIntrinsic{{1, -10, -5}, {2, 30, 40}}, true, IntrinsicWidths{20, 35}, false},
		{"max floored by min", []AtomicIntrinsic{{1, -10, -10}, {2, 30, 30}}, false, IntrinsicWidths{30, 30}, false},
		{"wide positive prefix", []AtomicIntrinsic{{1, 1500000000, 1500000000}, {2, 1500000000, 1500000000}, {3, -1500000000, -1500000000}}, true, IntrinsicWidths{1500000000, 1500000000}, false},
		{"wide negative prefix", []AtomicIntrinsic{{1, -1500000000, -1500000000}, {2, -1500000000, -1500000000}, {3, 1500000000, 1500000000}}, true, IntrinsicWidths{}, false},
		{"largest finite", []AtomicIntrinsic{{1, dimen.Fil - 1, dimen.Fil - 1}}, false, IntrinsicWidths{dimen.Fil - 1, dimen.Fil - 1}, false},
		{"completed overflow", []AtomicIntrinsic{{1, dimen.Fil - 1, dimen.Fil - 1}, {2, 1, 1}}, true, IntrinsicWidths{}, true},
		{"max overflow only", []AtomicIntrinsic{{1, 1, dimen.Fil - 1}, {2, 1, 1}}, true, IntrinsicWidths{}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text := ""
			var bounds []AtomicBoundary
			for n := range tc.values {
				text += "\uFFFC"
				if tc.join && n > 0 {
					bounds = append(bounds, AtomicBoundary{uint64(n * 3), ProhibitBoundary})
				}
			}
			owner := intrinsicOwner()
			p := intrinsicPrepare(t, owner, text, PrepareOptions{AtomicBoundaries: bounds})
			got, err := owner.MeasureIntrinsic(t.Context(), p, MeasureRequest{tc.values})
			if tc.bad {
				if got != (IntrinsicWidths{}) || !errors.Is(err, ErrIllegalArguments) {
					t.Fatal(got, err)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatal(got, err, "want", tc.want)
			}
		})
	}
}

func TestIntrinsicMinOverflowDespiteNegativeMaxSuffix(t *testing.T) {
	owner := intrinsicOwner()
	p := intrinsicPrepare(t, owner, "\uFFFC\uFFFC\uFFFC", PrepareOptions{AtomicBoundaries: []AtomicBoundary{{3, ProhibitBoundary}, {6, AllowBoundary}}})
	request := MeasureRequest{[]AtomicIntrinsic{{1, 1500000000, 1500000000}, {2, 1500000000, 1500000000}, {3, -1500000000, -1500000000}}}
	if got, err := owner.MeasureIntrinsic(t.Context(), p, request); got != (IntrinsicWidths{}) || !errors.Is(err, ErrIllegalArguments) {
		t.Fatal("out-of-domain min-content hidden by signed max-content", got, err)
	}
}

func TestIntrinsicIgnoresFillAndElasticity(t *testing.T) {
	for _, fill := range []KnotCore{{MaxW: dimen.Fill}, {W: 12345, MinW: 12345, MaxW: dimen.Fill}, {W: dimen.Fil, MinW: dimen.Fil, MaxW: dimen.Fill}} {
		for _, stretch := range []float64{0, 200} {
			owner := intrinsicOwner()
			owner.env.parFillSkip = fill
			owner.env.params.SpaceStretchPercent = stretch
			owner.env.params.SpaceShrinkPercent = 100
			owner.env.params.LeftSkip = KnotCore{W: 9 * dimen.BP, MinW: 9 * dimen.BP, MaxW: dimen.Fil}
			owner.env.params.RightSkip = owner.env.params.LeftSkip
			p := intrinsicPrepare(t, owner, "aa b", PrepareOptions{})
			got, err := owner.MeasureIntrinsic(t.Context(), p, MeasureRequest{})
			if err != nil || got != (IntrinsicWidths{10 * dimen.BP, 20 * dimen.BP}) {
				t.Fatal("fill, skips or elasticity affected intrinsic width", got, err)
			}
		}
	}
}

func TestIntrinsicNeutralAndTrimSemantics(t *testing.T) {
	// Internal prepared-material fixtures isolate neutral retention from Unicode
	// classification. Every synthetic source is empty: only the final compiler
	// trailer is excluded; earlier retained zero-source knots still contribute.
	for _, tc := range []struct {
		name  string
		knots []KnotCore
		want  dimen.DU
	}{
		{"neutral only", []KnotCore{{Kind: KTKern, W: 3, Penalty: MaxPenalty}}, 3},
		{"neutral does not shelter", []KnotCore{{Kind: KTGlue, W: 5, Flags: KFDiscardable, Penalty: MaxPenalty}, {Kind: KTKern, W: 3, Penalty: MaxPenalty}, {Kind: KTGlue, W: 7, Flags: KFDiscardable, Penalty: MaxPenalty}}, 3},
		{"neutral inside trimmed suffix", []KnotCore{{Kind: KTTextBox, W: 10, Penalty: MaxPenalty}, {Kind: KTGlue, W: 5, Flags: KFDiscardable, Penalty: MaxPenalty}, {Kind: KTKern, W: 3, Penalty: MaxPenalty}, {Kind: KTGlue, W: 7, Flags: KFDiscardable, Penalty: MaxPenalty}}, 13},
		{"interior discardable retained", []KnotCore{{Kind: KTTextBox, W: 10, Penalty: MaxPenalty}, {Kind: KTGlue, W: 5, Flags: KFDiscardable, Penalty: MaxPenalty}, {Kind: KTKern, W: 3, Penalty: MaxPenalty}, {Kind: KTGlue, W: 7, Flags: KFDiscardable, Penalty: MaxPenalty}, {Kind: KTTextBox, W: 10, Penalty: MaxPenalty}}, 35},
	} {
		t.Run(tc.name, func(t *testing.T) {
			owner := intrinsicOwner()
			p := intrinsicPrepare(t, owner, "", PrepareOptions{})
			p.knots = append(slices.Clone(tc.knots), p.knots[0])
			p.shaping.sources = make([]knotSource, len(p.knots))
			got, err := owner.MeasureIntrinsic(t.Context(), p, MeasureRequest{})
			if err != nil || got != (IntrinsicWidths{tc.want, tc.want}) {
				t.Fatal(got, err)
			}
			var line logicalLine
			for _, k := range tc.knots {
				line.knots = append(line.knots, logicalKnot{core: k})
			}
			if err := line.trimAndMeasure(); err != nil || line.naturalWidth != tc.want {
				t.Fatal("freeze trimming differs", line.naturalWidth, err)
			}
		})
	}
}

func TestIntrinsicValidationAndRecovery(t *testing.T) {
	owner := intrinsicOwner()
	p := intrinsicPrepare(t, owner, "\uFFFC\uFFFC", PrepareOptions{})
	valid := MeasureRequest{[]AtomicIntrinsic{{2, 30, 40}, {1, 10, 20}}}
	for _, atoms := range [][]AtomicIntrinsic{
		nil, {{1, 10, 20}}, {{1, 10, 20}, {2, 30, 40}, {3, 1, 2}}, {{0, 10, 20}, {2, 30, 40}}, {{1, 10, 20}, {1, 30, 40}}, {{1, 10, 20}, {9, 30, 40}},
		{{1, 20, 10}, {2, 30, 40}}, {{1, -dimen.Fil, 0}, {2, 30, 40}}, {{1, 0, dimen.Fil}, {2, 30, 40}}, {{1, dimen.Fill, dimen.Fill}, {2, 30, 40}},
	} {
		got, err := owner.MeasureIntrinsic(t.Context(), p, MeasureRequest{atoms})
		if got != (IntrinsicWidths{}) || !errors.Is(err, ErrIllegalArguments) {
			t.Fatal(atoms, got, err)
		}
	}
	for _, prep := range []*PreparedParagraph{nil, {}, intrinsicPrepare(t, intrinsicOwner(), "", PrepareOptions{})} {
		if got, err := owner.MeasureIntrinsic(t.Context(), prep, valid); got != (IntrinsicWidths{}) || !errors.Is(err, ErrIllegalArguments) {
			t.Fatal(got, err)
		}
	}
	if _, err := owner.MeasureIntrinsic(nil, p, valid); !errors.Is(err, ErrIllegalArguments) {
		t.Fatal(err)
	}
	if _, err := (*Khipukamayuq)(nil).MeasureIntrinsic(t.Context(), p, valid); !errors.Is(err, ErrVoidKhipukamayuq) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := (*Khipukamayuq)(nil).MeasureIntrinsic(ctx, nil, MeasureRequest{}); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation precedence", err)
	}
	// Cancellation during scanning rather than at entry must discard all results.
	ctx2 := &intrinsicCancelContext{Context: t.Context(), remaining: 8}
	if got, err := owner.MeasureIntrinsic(ctx2, p, valid); got != (IntrinsicWidths{}) || !errors.Is(err, context.Canceled) {
		t.Fatal(got, err)
	}
	want := IntrinsicWidths{30, 60}
	if got, err := owner.MeasureIntrinsic(t.Context(), p, valid); got != want || err != nil {
		t.Fatal("retry", got, err)
	}
	valid.Atomics[0].MinAdvance = 35
	if got, err := owner.MeasureIntrinsic(t.Context(), p, valid); got != (IntrinsicWidths{35, 60}) || err != nil {
		t.Fatal("stale atomic request", got, err)
	}
}

type intrinsicCancelContext struct {
	context.Context
	remaining int
}

func (c *intrinsicCancelContext) Err() error {
	c.remaining--
	if c.remaining <= 0 {
		return context.Canceled
	}
	return c.Context.Err()
}

func TestIntrinsicNoServicesOrMutation(t *testing.T) {
	for _, mode := range []HyphenationMode{NoHyphenation, AutomaticHyphenation} {
		policy := &testhyphenator{want: []HyphenationFragment{{PreBreak: "-"}}}
		calls := 0
		owner := NewKhipukamayuq(Config{Hyphenator: policy, Streaming: &StreamingConfig{
			Resolver: preparationResolver(func(_ styled.Style, d contracts.ResolvedRunStyle) (contracts.ResolvedRunStyle, error) {
				calls++
				return d, nil
			}), Defaults: preparationDefaults(),
			NewShaper: func() (contracts.StreamingShaper, error) { calls++; return helpers.NewSyntheticStreamingShaper(8) },
		}})
		p := intrinsicPrepare(t, owner, "ab\u00adcd\uFFFC", PrepareOptions{Hyphenation: mode})
		before := calls
		knots := slices.Clone(p.knots)
		glyphs := slices.Clone(p.shaping.glyphs)
		for range 3 {
			got, err := owner.MeasureIntrinsic(t.Context(), p, MeasureRequest{[]AtomicIntrinsic{{1, 10, 30}}})
			if mode == NoHyphenation {
				if err != nil {
					t.Fatal(err)
				}
			} else if got != (IntrinsicWidths{}) || !errors.Is(err, ErrUnsupportedPolicy) {
				t.Fatal(got, err)
			}
		}
		if calls != before || policy.calls != 0 || !reflect.DeepEqual(knots, p.knots) || !reflect.DeepEqual(glyphs, p.shaping.glyphs) {
			t.Fatal("measurement called services or mutated preparation")
		}
	}
}

func TestIntrinsicCheckedWideAddition(t *testing.T) {
	for _, tc := range []struct {
		sum, delta, want int64
		overflow         bool
	}{
		{math.MaxInt64 - 1, 1, math.MaxInt64, false}, {math.MinInt64 + 1, -1, math.MinInt64, false},
		{math.MaxInt64, 1, 0, true}, {math.MinInt64, -1, 0, true}, {math.MinInt64, math.MaxInt64, -1, false},
	} {
		got, err := addIntrinsicAdvance(tc.sum, tc.delta)
		if got != tc.want || (tc.overflow && !errors.Is(err, ErrIllegalArguments)) || (!tc.overflow && err != nil) {
			t.Fatal(tc, got, err)
		}
	}
}
