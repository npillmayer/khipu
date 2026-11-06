package khipu_test

import (
	"errors"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/uax/bidi"
)

// External callers must be able to construct a configured owner and have their
// shaper used. Error propagation exercises that boundary without depending on
// the encoder's still-unsettled segmentation and paragraph-trailer behavior.
func TestPublicConfigShaperError(t *testing.T) {
	wantErr := errors.New("shaper failed")
	shaper := newEncodingScriptStream([]contracts.ShapedGlyph{{Cluster: 0, XAdvance: 500}}, wantErr)
	cfg := encodingStreamingConfig()
	cfg.NewShaper = func() (contracts.StreamingShaper, error) { return shaper, nil }
	kq := khipu.NewKhipukamayuq(khipu.Config{
		Streaming: cfg,
		Params:    &khipu.Params{Hyphenpenalty: 37},
	})
	text := styled.TextFromString("breaking")
	para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), bidi.LeftToRight, nil)
	if err != nil {
		t.Fatalf("create paragraph: %v", err)
	}
	khp, err := kq.EncodeParagraph(t.Context(), para)
	if !errors.Is(err, wantErr) || khp != nil {
		t.Fatalf("expected shaper error without partial paragraph, got %v, %v", khp, err)
	}
	if shaper.input != "breaking" || shaper.params.Font != cfg.Defaults.Font {
		t.Fatalf("configured shaper did not receive source and style: %+v", shaper)
	}
}

func TestPublicConfigWithoutHyphenator(t *testing.T) {
	kq := khipu.NewKhipukamayuq(khipu.Config{})
	khp := &khipu.Khipu{
		W:       []dimen.DU{0},
		MinW:    []dimen.DU{0},
		MaxW:    []dimen.DU{0},
		Penalty: []khipu.Penalty{0},
		Pos:     []uint64{0},
		Len:     []uint16{0},
		Kind:    []khipu.KnotType{khipu.KTTextBox},
		Flags:   []khipu.KnotFlags{0},
	}
	candidates, err := kq.DiscretionaryCandidates(khp, 0)
	if err != nil || len(candidates) != 0 {
		t.Fatalf("expected no discovery without a hyphenator, got %v, %v", candidates, err)
	}
	cached := khipu.DiscretionaryCandidate{Variant: 7}
	if !khp.AddDiscretionaryCandidate(0, cached) {
		t.Fatal("cannot add cached candidate")
	}
	candidates, err = kq.DiscretionaryCandidates(khp, 0)
	if err != nil || len(candidates) != 1 || candidates[0] != cached {
		t.Fatalf("expected cached candidate without a hyphenator, got %v, %v", candidates, err)
	}
}
