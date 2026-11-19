package khipu

import (
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
)

func TestPreparedExplicitLazyImportAndIsolation(t *testing.T) {
	for _, layoutFirst := range []bool{false, true} {
		t.Run(map[bool]string{false: "measure first", true: "layout first"}[layoutFirst], func(t *testing.T) {
			material := []HyphenationFragment{{PreBreak: "-"}}
			o, h, shapes := explicitOwner(material)
			p := intrinsicPrepare(t, o, "éa\u00adcd", PrepareOptions{Hyphenation: ExplicitHyphenation})
			before := *shapes
			bind := func() *FixtureKhipu {
				t.Helper()
				k, err := o.BindParagraphFixture(p, nil)
				if err != nil {
					t.Fatal(err)
				}
				if len(k.DiscretionaryCandidates) != 0 || len(k.discretionaryEmpty) != 0 {
					t.Fatal("eager import")
				}
				return k
			}
			early := bind()
			measure := func() {
				t.Helper()
				got, err := o.MeasureIntrinsicFixture(p, MeasureRequest{})
				if err != nil || got != (IntrinsicWidths{15 * dimen.BP, 20 * dimen.BP}) {
					t.Fatal(got, err)
				}
			}
			candidates := func(k *FixtureKhipu) []DiscretionaryCandidate {
				t.Helper()
				c, err := o.DiscoverFixture(k, 0)
				if err != nil || len(c) != 1 {
					t.Fatal(c, err)
				}
				return c
			}
			if layoutFirst {
				candidates(early)
			}
			measure()
			c := candidates(early)
			if c[0].SourceSplit != (SourceSplit{5, true}) || c[0].PreBreak.Pos != 0 || c[0].PreBreak.Len != 5 || c[0].PostBreak.Pos != 5 || c[0].PostBreak.Len != 2 {
				t.Fatal("lost original UTF-8 provenance", c)
			}
			later := bind()
			want := slices.Clone(candidates(later))
			box := later.shaping.boxes[want[0].preBox-1]
			wantGlyphs := slices.Clone(later.shaping.glyphs[box.glyphs.start:box.glyphs.end])
			// Imported records and clusters are detached from both cache and other bindings.
			old := early.shaping.boxes[c[0].preBox-1]
			early.shaping.glyphs[old.glyphs.start].GlyphID = 999
			early.shaping.clusters[old.clusters.start].source.byteStart = 999
			c[0].PreBreak.W = 0
			material[0].PreBreak = "changed caller storage"
			measure()
			if !reflect.DeepEqual(want, candidates(later)) || !reflect.DeepEqual(wantGlyphs, later.shaping.glyphs[box.glyphs.start:box.glyphs.end]) {
				t.Fatal("aliased imports")
			}
			third := bind()
			thirdC := candidates(third)
			if !reflect.DeepEqual(want, thirdC) {
				t.Fatal("cache corrupted", thirdC)
			}
			local := bind()
			manual := DiscretionaryCandidate{Variant: 99, PreBreak: KnotCore{W: 123}}
			local.AddDiscretionaryCandidate(0, manual)
			if got, err := o.DiscoverFixture(local, 0); err != nil || !reflect.DeepEqual(got, []DiscretionaryCandidate{manual}) {
				t.Fatal("manual precedence", got, err)
			}
			unwireKhipu(early)
			if early.prepared != nil {
				t.Fatal("release retained preparation")
			}
			measure()
			candidates(third)
			if h.calls != 1 || *shapes != before+2 {
				t.Fatal("cache miss", h.calls, *shapes, before)
			}
		})
	}
}

func TestPreparedExplicitEmptyCache(t *testing.T) {
	o, h, shapes := explicitOwner(nil)
	p := intrinsicPrepare(t, o, "ab\u00adcd", PrepareOptions{Hyphenation: ExplicitHyphenation})
	before := *shapes
	for range 3 {
		k, err := o.BindParagraphFixture(p, nil)
		if err != nil {
			t.Fatal(err)
		}
		if c, err := o.DiscoverFixture(k, 0); err != nil || len(c) != 0 {
			t.Fatal(c, err)
		}
		if _, err := o.MeasureIntrinsicFixture(p, MeasureRequest{}); err != nil {
			t.Fatal(err)
		}
	}
	if b, ok := p.explicit[0]; !ok || len(b.replacements) != 0 || h.calls != 1 || *shapes != before {
		t.Fatal("empty result not cached")
	}
}

func TestPreparedExplicitUnsupportedMeasurementPreservesLayoutBatch(t *testing.T) {
	for _, tc := range []struct {
		text     string
		material []HyphenationFragment
		count    int
	}{
		{"ab\u00adcd\u00adef", []HyphenationFragment{{PreBreak: "-"}}, 2},
		{"ab\u00ad\u00adcd", []HyphenationFragment{{PreBreak: "-"}}, 2},
		{"ab\u00adcd", []HyphenationFragment{{PreBreak: "-"}, {PreBreak: "-"}}, 2},
		{"ab\u00adcd\u00adef", nil, 0},
	} {
		o, h, shapes := explicitOwner(tc.material)
		p := intrinsicPrepare(t, o, tc.text, PrepareOptions{Hyphenation: ExplicitHyphenation})
		if got, err := o.MeasureIntrinsicFixture(p, MeasureRequest{}); got != (IntrinsicWidths{}) || !errors.Is(err, ErrUnsupportedPolicy) {
			t.Fatal(got, err)
		}
		a, err := o.BindParagraphFixture(p, nil)
		if err != nil {
			t.Fatal(err)
		}
		ca, err := o.DiscoverFixture(a, 0)
		if err != nil || len(ca) != tc.count {
			t.Fatal(ca, err)
		}
		shapesBefore, callsBefore := *shapes, h.calls
		for range 2 {
			if got, err := o.MeasureIntrinsicFixture(p, MeasureRequest{}); got != (IntrinsicWidths{}) || !errors.Is(err, ErrUnsupportedPolicy) {
				t.Fatal(got, err)
			}
			b, err := o.BindParagraphFixture(p, nil)
			if err != nil {
				t.Fatal(err)
			}
			cb, err := o.DiscoverFixture(b, 0)
			if err != nil || !reflect.DeepEqual(ca, cb) {
				t.Fatal(cb, err)
			}
		}
		if *shapes != shapesBefore || h.calls != callsBefore {
			t.Fatal("cached rejected batch was discarded")
		}
	}
}

func TestPreparedExplicitImportValidation(t *testing.T) {
	o, _, _ := explicitOwner([]HyphenationFragment{{PreBreak: "-"}})
	a := intrinsicPrepare(t, o, "ab\u00adcd", PrepareOptions{Hyphenation: ExplicitHyphenation})
	b := intrinsicPrepare(t, o, "ab\u00adcd", PrepareOptions{Hyphenation: ExplicitHyphenation})
	if _, e := MeasureIntrinsic(a.material, a.discovery, MeasureRequest{}); e != nil {
		t.Fatal(e)
	}
	binding, e := BindParagraph(a.material, nil)
	if e != nil {
		t.Fatal(e)
	}
	request := BreakRequest{Binding: binding, Discovery: b.discovery, Geometry: GeometryRequest{Shape: fixtureShape{}}}
	if e = request.Validate(); !errors.Is(e, ErrIllegalArguments) {
		t.Fatal("foreign discovery", e)
	}
	request.Discovery = a.discovery
	if e = request.Validate(); e != nil {
		t.Fatal(e)
	}
	if got, e := a.discovery.Alternatives(0); e != nil || len(got.Candidates()) != 1 {
		t.Fatal(got, e)
	}
}

func TestPreparedExplicitFeaturesAndZeroWidthContent(t *testing.T) {
	for _, limited := range []bool{false, true} {
		o, _, _ := explicitOwner([]HyphenationFragment{{PreBreak: "-"}})
		p := intrinsicPrepare(t, o, "ab\u00adcd", PrepareOptions{Hyphenation: ExplicitHyphenation})
		feature := contracts.ShapeFeature{Enabled: true, End: 5}
		if limited {
			feature.Start = 1
			feature.End = 2
		}
		// Synthetic base shaping rejects features. Seed the retained metadata and use
		// a recording backend for replacements, as in the existing provider tests.
		p.shaping.contexts[0].params.shaping.Features = []contracts.ShapeFeature{feature}
		var streams []*fragmentRecordingShaper
		p.discovery.recipe.env.streaming.NewShaper = func() (contracts.StreamingShaper, error) {
			s := &fragmentRecordingShaper{pipelineShaper: newPipelineShaper()}
			streams = append(streams, s)
			return s, nil
		}
		got, err := o.MeasureIntrinsicFixture(p, MeasureRequest{})
		if limited {
			if got != (IntrinsicWidths{}) || !errors.Is(err, contracts.ErrIllegalArguments) || len(p.explicit) != 0 || len(streams) != 0 {
				t.Fatal(got, err)
			}
			continue
		}
		if err != nil || got != (IntrinsicWidths{0, 20 * dimen.BP}) || len(streams) != 2 {
			t.Fatal(got, err)
		}
		for _, s := range streams {
			if !reflect.DeepEqual(s.params.Features, []contracts.ShapeFeature{{Enabled: true}}) {
				t.Fatal(s.params.Features)
			}
		}
		k, err := o.BindParagraphFixture(p, nil)
		if err != nil {
			t.Fatal(err)
		}
		c, err := o.DiscoverFixture(k, 0)
		if err != nil {
			t.Fatal(err)
		}
		box := k.shaping.boxes[c[0].preBox-1]
		k.shaping.contexts[box.context].params.shaping.Features[0].Enabled = false
		if p.shaping.contexts[0].params.shaping.Features[0] != feature || !p.explicit[0].replacements[0].pair.pre.params.shaping.Features[0].Enabled {
			t.Fatal("aliased features")
		}
		// Zero-width post is still content, protecting a neighboring discardable
		// from trailing-edge trimming. Use the span evaluator's same content rule.
		post := KnotCore{Kind: KTTextBox, W: c[0].PostBreak.W}
		w, err := intrinsicKnotSpan([]KnotCore{{Kind: KTTextBox, W: 3}, {Kind: KTGlue, W: 7, Flags: KFDiscardable}, post}, nil)
		if err != nil || w != 10 {
			t.Fatal("zero-width replacement lost content identity", w, err)
		}
	}
}

func TestPreparedExplicitWholeBatchFailure(t *testing.T) {
	o, h, _ := explicitOwner([]HyphenationFragment{{PreBreak: "-"}, {PreBreak: "="}})
	p := intrinsicPrepare(t, o, "ab\u00adcd", PrepareOptions{Hyphenation: ExplicitHyphenation})
	k, err := o.BindParagraphFixture(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("last fragment failed")
	shapes := 0
	p.discovery.recipe.env.streaming.NewShaper = func() (contracts.StreamingShaper, error) {
		shapes++
		if shapes == 4 {
			return nil, sentinel
		}
		return helpers.NewSyntheticStreamingShaper(8)
	}
	before := k.shaping
	if c, err := o.DiscoverFixture(k, 0); len(c) != 0 || !errors.Is(err, sentinel) {
		t.Fatal(c, err)
	}
	if len(p.explicit) != 0 || len(k.DiscretionaryCandidates) != 0 || !reflect.DeepEqual(before, k.shaping) {
		t.Fatal("partial batch published")
	}
	if c, err := o.DiscoverFixture(k, 0); len(c) != 2 || err != nil || h.calls != 2 {
		t.Fatal("retry", c, err, h.calls)
	}
	if _, err := o.MeasureIntrinsicFixture(p, MeasureRequest{}); !errors.Is(err, ErrUnsupportedPolicy) {
		t.Fatal(err)
	}
	if h.calls != 2 || shapes != 8 {
		t.Fatal("measurement discarded layout batch")
	}
}
