package khipu_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
	"github.com/npillmayer/khipu/linebreak"
)

func TestRectangularLineShape(t *testing.T) {
	var shape khipu.LineShape = linebreak.RectangularParShape(100 * dimen.PT)
	if got := shape.LineLength(1); got != 100*dimen.PT {
		t.Fatalf("rectangular shape width: %v", got)
	}
}

// Exercise canonical service contracts and helpers through paragraph encoding.
func TestIntegrationAcrossServicePackages(t *testing.T) {
	var face *helpers.SyntheticFont = helpers.NewSyntheticFont()
	var resolver *helpers.DefaultStyleResolver = helpers.NewDefaultStyleResolver()
	style, err := resolver.ResolveStyle(nil, contracts.ResolvedRunStyle{
		Font: face, EmSize: 10 * dimen.PT, Spacing: contracts.SpacingFixed,
	})
	if err != nil {
		t.Fatal(err)
	}
	var metrics contracts.FontMetrics
	metrics, err = (contracts.FontFace)(face).Metrics()
	if err != nil || metrics.UnitsPerEm != 1000 {
		t.Fatalf("font does not satisfy the contract: %+v, %v", metrics, err)
	}
	stream, err := helpers.NewSyntheticStreamingShaper(1)
	if err != nil {
		t.Fatal(err)
	}
	var shaper contracts.StreamingShaper = stream
	if err := shaper.Shape(t.Context(), contracts.ShapeParams{Font: style.Font},
		strings.NewReader("é"), contracts.ShapeBufferOptions{Flush: contracts.ShapeFlushCluster}); err != nil {
		t.Fatal(err)
	}
	if err := shaper.Finish(); err != nil {
		t.Fatal(err)
	}
	var glyph contracts.ShapedGlyph = <-shaper.Glyphs()
	if glyph.GlyphID != uint32('é')+1 || glyph.XAdvance != 500 {
		t.Fatalf("unexpected shaped glyph: %+v", glyph)
	}
	if err := shaper.SetInitialCluster(0); !errors.Is(err, helpers.ErrShaperClosed) {
		t.Fatalf("closed error identity changed: %v", err)
	}
	if khipu.ErrIllegalArguments != contracts.ErrIllegalArguments {
		t.Fatal("error identities changed across packages")
	}
	if _, err := helpers.NewSyntheticStreamingShaper(-1); !errors.Is(err, khipu.ErrIllegalArguments) {
		t.Fatalf("helper errors no longer match the root sentinel: %v", err)
	}
	owner := khipu.NewKhipukamayuq(khipu.Config{Streaming: &khipu.StreamingConfig{
		NewShaper: func() (contracts.StreamingShaper, error) { return helpers.NewSyntheticStreamingShaper(0) },
		Resolver:  resolver, Defaults: contracts.ResolvedRunStyle{Font: face, EmSize: 14 * dimen.PT},
	}})
	paragraph, err := owner.EncodeParagraph(t.Context(), encodingParagraph(t, "abc"))
	if err != nil || paragraph.KnotByIndex(0).W != 21*dimen.PT {
		t.Fatalf("streaming helper integration failed: %v, %v", paragraph, err)
	}
}
