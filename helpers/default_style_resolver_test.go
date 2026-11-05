package helpers_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
	"golang.org/x/text/language"
)

// Resolution must not query a backend or reject a foreign face by type.
type unqueriedFont struct{}

func (unqueriedFont) Metrics() (contracts.FontMetrics, error) { panic("unexpected metrics query") }
func (unqueriedFont) Scale(dimen.DU) (contracts.FontScale, error) {
	panic("unexpected scale query")
}

func TestDefaultStyleResolverCopiesCompleteDefaults(t *testing.T) {
	resolver := helpers.NewDefaultStyleResolver()
	defaults := contracts.ResolvedRunStyle{
		Font: unqueriedFont{}, EmSize: 10 * dimen.PT,
		Script: language.MustParseScript("Latn"), Language: language.English,
		Spacing:  contracts.SpacingElastic,
		Features: []contracts.ShapeFeature{{Tag: [4]byte{'l', 'i', 'g', 'a'}, Enabled: true, Start: 1, End: 3}},
	}
	first, err := resolver.ResolveStyle(nil, defaults)
	if err != nil || !reflect.DeepEqual(first, defaults) {
		t.Fatalf("did not preserve defaults: %+v, %v", first, err)
	}
	second, err := resolver.ResolveStyle(nil, defaults)
	if err != nil {
		t.Fatal(err)
	}
	defaults.Features[0].Start = 99
	if first.Features[0].Start != 1 || second.Features[0].Start != 1 {
		t.Fatal("caller mutation changed returned features")
	}
	first.Features[0].End = 77
	if second.Features[0].End != 3 || defaults.Features[0].End != 3 {
		t.Fatal("results share feature storage")
	}
	// No inheritance from preceding results, and no invented script/language.
	plain := contracts.ResolvedRunStyle{Font: helpers.SyntheticFont{}, EmSize: 1}
	got, err := (helpers.DefaultStyleResolver{}).ResolveStyle(nil, plain)
	if err != nil || !reflect.DeepEqual(got, plain) {
		t.Fatalf("zero properties were replaced: %+v, %v", got, err)
	}
}

func TestDefaultStyleResolverRejectsInvalidInput(t *testing.T) {
	valid := contracts.ResolvedRunStyle{Font: helpers.NewSyntheticFont(), EmSize: 10 * dimen.PT}
	var nilSynthetic *helpers.SyntheticFont
	var nilForeign *unqueriedFont
	for _, tc := range []struct {
		name   string
		style  styled.Style
		change func(*contracts.ResolvedRunStyle)
	}{
		{"application style", testStyle("bold"), func(*contracts.ResolvedRunStyle) {}},
		{"nil font", nil, func(s *contracts.ResolvedRunStyle) { s.Font = nil }},
		{"typed nil synthetic font", nil, func(s *contracts.ResolvedRunStyle) { s.Font = nilSynthetic }},
		{"typed nil foreign font", nil, func(s *contracts.ResolvedRunStyle) { s.Font = nilForeign }},
		{"zero size", nil, func(s *contracts.ResolvedRunStyle) { s.EmSize = 0 }},
		{"negative size", nil, func(s *contracts.ResolvedRunStyle) { s.EmSize = -1 }},
		{"unknown spacing", nil, func(s *contracts.ResolvedRunStyle) { s.Spacing = 42 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defaults := valid
			tc.change(&defaults)
			got, err := (helpers.DefaultStyleResolver{}).ResolveStyle(tc.style, defaults)
			if !errors.Is(err, contracts.ErrIllegalArguments) || !reflect.DeepEqual(got, contracts.ResolvedRunStyle{}) {
				t.Fatalf("got %+v, %v; want zero result and invalid arguments", got, err)
			}
		})
	}
}
