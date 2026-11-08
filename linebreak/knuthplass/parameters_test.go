package knuthplass

import (
	"reflect"
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
)

func TestParameterProfiles(t *testing.T) {
	standard := Parameters{
		PreTolerance: 100, Tolerance: 200, LinePenalty: 10,
		FinalHyphenDemerits: 10000, ParFillSkip: glue(0, 0, dimen.Fill),
	}
	tolerant := standard
	tolerant.Tolerance, tolerant.FinalHyphenDemerits = 5000, 50
	tolerant.EmergencyStretch = 50 * dimen.BP
	for _, tc := range []struct {
		name       string
		makeParams func() *Parameters
		want       Parameters
		passes     int
	}{
		{"default", NewDefaultParameters, standard, 2},
		{"tolerant", NewTolerantParameters, tolerant, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := tc.makeParams()
			if !reflect.DeepEqual(*p, tc.want) {
				t.Fatalf("profile changed: %+v, want %+v", *p, tc.want)
			}
			passes, err := paragraphPasses(p)
			if err != nil || len(passes) != tc.passes || !passes[len(passes)-1].final {
				t.Fatalf("incorrect pass policy: %+v, %v", passes, err)
			}
			p.Tolerance, p.ParFillSkip.MaxW = 1, 0
			if !reflect.DeepEqual(*tc.makeParams(), tc.want) {
				t.Fatal("constructor shares mutable defaults")
			}
		})
	}
}

func TestNilParametersUseIndependentDefaultProfile(t *testing.T) {
	want := NewDefaultParameters()
	wantTolerant := NewTolerantParameters()
	layout := DefaultLayout(linebreak.RectangularParShape(150 * dimen.PT))
	ownerInput := newKhipu([]node{
		{w: 20, minw: 20, maxw: 20, p: InfinityDemerits, kind: khipu.KTTextBox},
		{maxw: dimen.Fill, p: InfinityMerits, kind: khipu.KTGlue},
	})
	for _, independent := range []*Parameters{NewDefaultParameters(), NewTolerantParameters()} {
		independent.EmergencyStretch = dimen.Fil
		if !reflect.DeepEqual(*NewDefaultParameters(), *want) ||
			!reflect.DeepEqual(*NewTolerantParameters(), *wantTolerant) ||
			!reflect.DeepEqual(*newLinebreaker(layout, nil).params, *want) {
			t.Fatal("profile mutation affected independent defaults")
		}
		got, err := selectParagraphBreaks(ownerInput, layout, nil)
		explicit, explicitErr := selectParagraphBreaks(ownerInput, layout, want)
		if err != nil || explicitErr != nil || !got.complete || !reflect.DeepEqual(got, explicit) {
			t.Fatalf("nil parameters differ from explicit default profile: %+v, %v; %+v, %v", got, err, explicit, explicitErr)
		}
	}
}
