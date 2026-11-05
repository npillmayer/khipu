package khipu

import (
	"reflect"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/helpers"
)

func eligibilityParagraph(t *testing.T, p *styled.Paragraph) *Khipu {
	t.Helper()
	owner := NewKhipukamayuq(Config{Streaming: &StreamingConfig{
		Resolver:  preparationResolver(func(_ styled.Style, d contracts.ResolvedRunStyle) (contracts.ResolvedRunStyle, error) { return d, nil }),
		Defaults:  preparationDefaults(),
		NewShaper: func() (contracts.StreamingShaper, error) { return helpers.NewSyntheticStreamingShaper(0) },
	}})
	k, err := owner.EncodeParagraphStreaming(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestAutomaticEligibilityCompleteSegments(t *testing.T) {
	for _, text := range []string{"alpha", "alpha beta", "éclair", "e\u0301lan", "אבגד", "can't", "123", "."} {
		t.Run(text, func(t *testing.T) {
			k := eligibilityParagraph(t, preparationParagraph(t, text))
			textboxes := 0
			for i, kind := range k.Kind {
				want := kind == KTTextBox
				if want {
					textboxes++
				}
				if got := k.automaticHyphenationEligible(i); got != want {
					t.Fatalf("knot %d: eligibility=%v, want %v", i, got, want)
				}
			}
			if textboxes == 0 {
				t.Fatal("fixture has no textboxes")
			}
			if len(k.DiscretionaryCandidates) != 0 || len(k.SelectedDiscretionaries) != 0 {
				t.Fatal("eligibility discovered candidates")
			}
		})
	}
}

func TestAutomaticEligibilityRejectsStructuralWordFragments(t *testing.T) {
	for _, text := range []string{"metafinanz", "abcאבג", "ab\u2066cd\u2069ef"} {
		t.Run(text, func(t *testing.T) {
			p := preparationParagraph(t, text)
			if text == "metafinanz" {
				if _, err := p.Style(intersectionStyle("logo"), 0, 4); err != nil {
					t.Fatal(err)
				}
			}
			k := eligibilityParagraph(t, p)
			boxes := 0
			for i, kind := range k.Kind {
				if kind == KTTextBox {
					boxes++
				}
				if k.automaticHyphenationEligible(i) {
					t.Fatalf("accepted word fragment %d: %+v", i, k.shaping.sources[i])
				}
			}
			if boxes < 2 {
				t.Fatal("fixture did not split word")
			}
		})
	}
}

func TestAutomaticEligibilityStyleBoundaryAtWordEnd(t *testing.T) {
	p := preparationParagraph(t, "alpha beta")
	if _, err := p.Style(intersectionStyle("first"), 0, 5); err != nil {
		t.Fatal(err)
	}
	k := eligibilityParagraph(t, p)
	for i, kind := range k.Kind {
		if kind == KTTextBox && !k.automaticHyphenationEligible(i) {
			t.Fatal("rejected complete styled word", i)
		}
	}
}

func TestAutomaticEligibilityRejectsExplicitAndLegacy(t *testing.T) {
	for _, text := range []string{"ab\u00adcd", "ab\u00ad", "\u00adcd", "\u00ad", ""} {
		k := eligibilityParagraph(t, preparationParagraph(t, text))
		for i := range k.Kind {
			if k.automaticHyphenationEligible(i) {
				t.Fatal("accepted explicit or incomplete source", text, i)
			}
		}
	}
	legacy, err := NewKhipukamayuq(Config{}).EncodeParagraph(preparationParagraph(t, "alphabet"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if legacy.automaticHyphenationEligible(0) || (*Khipu)(nil).automaticHyphenationEligible(0) {
		t.Fatal("accepted missing streaming metadata")
	}
}

func TestAutomaticEligibilityRejectsClusterSpanningWords(t *testing.T) {
	// A backend can return a cluster spanning multiple default word segments;
	// cluster safety prevented knot splitting, but does not make this one word.
	a, g := candidateFixture(t, "alpha beta")
	knots := accumulateTestGlyphs(t, a, g, []contracts.ShapedGlyph{{Cluster: 0, XAdvance: 500}})
	k, err := NewKhipukamayuq(Config{}).publishCandidateParagraph(&candidateParagraph{a.para, a.runs, a.boundaries, knots})
	if err != nil {
		t.Fatal(err)
	}
	if len(k.Kind) != 2 || k.automaticHyphenationEligible(0) {
		t.Fatal("accepted merged word segments")
	}
}

func TestAutomaticEligibilityMetadataAndPurity(t *testing.T) {
	for _, mode := range []string{"valid", "negative", "outside", "box id", "context", "context range", "replacement", "mismatch", "byte range", "discardable"} {
		t.Run(mode, func(t *testing.T) {
			k := eligibilityParagraph(t, preparationParagraph(t, "alpha"))
			at := 0
			switch mode {
			case "negative":
				at = -1
			case "outside":
				at = len(k.Kind)
			case "box id":
				k.boxRefs[0] = 99
			case "context":
				k.shaping.boxes[0].context = 99
			case "context range":
				k.shaping.contexts[0].source.scalarStart++
			case "replacement":
				k.shaping.boxes[0].replacement = &replacementInput{}
			case "mismatch":
				k.shaping.sources[0].source.byteEnd--
			case "byte range":
				k.shaping.boxes[0].source.byteEnd--
				k.shaping.sources[0].source.byteEnd--
			case "discardable":
				k.SetKnotFlags(0, KFDiscardable)
			}
			before := k.KnotByIndex(0)
			bounds := append([]sourceBoundary(nil), k.shaping.boundaries...)
			if k.automaticHyphenationEligible(at) != (mode == "valid") {
				t.Fatal("unexpected eligibility")
			}
			if k.KnotByIndex(0) != before || !reflect.DeepEqual(bounds, k.shaping.boundaries) || len(k.DiscretionaryCandidates) != 0 {
				t.Fatal("eligibility mutated paragraph")
			}
		})
	}
}
