package khipu_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
	"github.com/npillmayer/khipu/linebreak"
)

func TestParShapeAliasesLineShape(t *testing.T) {
	if reflect.TypeFor[linebreak.ParShape]() != reflect.TypeFor[khipu.LineShape]() {
		t.Fatal("ParShape must retain LineShape type identity")
	}
	var shape khipu.LineShape = linebreak.RectangularParShape(100 * dimen.PT)
	if got := shape.LineLength(1); got != 100*dimen.PT {
		t.Fatalf("rectangular shape width: %v", got)
	}
}

// Exercise old constructor/type names across the new package boundaries. In
// particular, aliases must retain the concrete face identity checked by shaping.
func TestCompatibilityAcrossServicePackages(t *testing.T) {
	var face *helpers.SyntheticFont = khipu.NewSyntheticFont()
	var resolver *helpers.DefaultStyleResolver = khipu.NewDefaultStyleResolver()
	style, err := resolver.ResolveStyle(nil, khipu.ResolvedRunStyle{
		Font: face, EmSize: 10 * dimen.PT, Spacing: khipu.SpacingFixed,
	})
	if err != nil {
		t.Fatal(err)
	}
	var metrics contracts.FontMetrics
	metrics, err = (khipu.FontFace)(face).Metrics()
	if err != nil || metrics.UnitsPerEm != 1000 {
		t.Fatalf("font alias does not preserve the contract: %+v, %v", metrics, err)
	}
	stream, err := khipu.NewSyntheticStreamingShaper(1)
	if err != nil {
		t.Fatal(err)
	}
	var shaper contracts.StreamingShaper = stream
	if err := shaper.Shape(t.Context(), khipu.ShapeParams{Font: style.Font},
		strings.NewReader("é"), khipu.ShapeBufferOptions{Flush: khipu.ShapeFlushCluster}); err != nil {
		t.Fatal(err)
	}
	if err := shaper.Finish(); err != nil {
		t.Fatal(err)
	}
	var glyph khipu.ShapedGlyph = <-shaper.Glyphs()
	if glyph.GlyphID != uint32('é')+1 || glyph.XAdvance != 500 {
		t.Fatalf("unexpected glyph through compatibility names: %+v", glyph)
	}
	if err := shaper.SetInitialCluster(0); !errors.Is(err, khipu.ErrShaperClosed) {
		t.Fatalf("closed error identity changed: %v", err)
	}
	if khipu.ErrShaperBusy != helpers.ErrShaperBusy || khipu.ErrShaperClosed != helpers.ErrShaperClosed || khipu.ErrIllegalArguments != contracts.ErrIllegalArguments {
		t.Fatal("error identities changed across packages")
	}
	if _, err := helpers.NewSyntheticStreamingShaper(-1); !errors.Is(err, khipu.ErrIllegalArguments) {
		t.Fatalf("helper errors no longer match the root sentinel: %v", err)
	}
	owner := khipu.NewKhipukamayuq(khipu.Config{Streaming: &khipu.StreamingConfig{
		NewShaper: func() (khipu.StreamingShaper, error) { return khipu.NewSyntheticStreamingShaper(0) },
		Resolver:  resolver, Defaults: khipu.ResolvedRunStyle{Font: face, EmSize: 14 * dimen.PT},
	}})
	paragraph, err := owner.EncodeParagraph(t.Context(), encodingParagraph(t, "abc"))
	if err != nil || paragraph.KnotByIndex(0).W != 21*dimen.PT {
		t.Fatalf("streaming helper compatibility failed: %v, %v", paragraph, err)
	}
}
