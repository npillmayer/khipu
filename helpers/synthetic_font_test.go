package helpers_test

import (
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
)

func TestSyntheticFontMetrics(t *testing.T) {
	want := contracts.FontMetrics{UnitsPerEm: 1000, Ascent: 800, Descent: -200}
	for _, face := range []contracts.FontFace{helpers.SyntheticFont{}, helpers.NewSyntheticFont()} {
		got, err := face.Metrics()
		if err != nil || got != want {
			t.Fatalf("metrics = %+v, %v; want %+v", got, err, want)
		}
		got.UnitsPerEm = 1
		again, err := face.Metrics()
		if err != nil || again != want {
			t.Fatalf("caller mutation changed face metrics: %+v, %v", again, err)
		}
	}
}

func TestSyntheticFontScale(t *testing.T) {
	face := helpers.NewSyntheticFont()
	for _, tc := range []struct {
		name   string
		emSize dimen.DU
		units  int32
		want   float64
	}{
		{"half em", 10 * dimen.PT, 500, float64(5 * dimen.PT)},
		{"negative descent", 10 * dimen.PT, -200, float64(-2 * dimen.PT)},
		{"zero advance", 10 * dimen.PT, 0, 0},
		{"fractional positive", 125, 3, 0.375},
		{"fractional negative", 125, -3, -0.375},
		{"smallest em", 1, 1, 0.001},
		{"largest em", dimen.DU(math.MaxInt32), 1000, float64(math.MaxInt32)},
		{"largest input", 1000, math.MaxInt32, float64(math.MaxInt32)},
		{"smallest input", 1000, math.MinInt32, float64(math.MinInt32)},
		{"beyond DU range", 2000, math.MaxInt32, 4294967294},
		{"below DU range", 2000, math.MinInt32, -4294967296},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scale, err := face.Scale(tc.emSize)
			if err != nil {
				t.Fatal(err)
			}
			if got := scale.ToDU(tc.units); got != tc.want {
				t.Fatalf("ToDU(%d) = %v; want %v", tc.units, got, tc.want)
			}
			// One face may serve several runs without changing existing scales.
			if _, err := face.Scale(17); err != nil {
				t.Fatal(err)
			}
			if got := scale.ToDU(tc.units); got != tc.want {
				t.Fatalf("creating another scale changed conversion: %v", got)
			}
		})
	}
	for _, size := range []dimen.DU{0, -1, dimen.DU(math.MinInt32)} {
		if scale, err := face.Scale(size); scale != nil || !errors.Is(err, contracts.ErrIllegalArguments) {
			t.Errorf("Scale(%d) = %v, %v; want nil and ErrIllegalArguments", size, scale, err)
		}
	}
}

func ExampleSyntheticFont() {
	face := helpers.NewSyntheticFont()
	metrics, err := face.Metrics()
	if err != nil {
		panic(err)
	}
	scale, err := face.Scale(10 * dimen.PT)
	if err != nil {
		panic(err)
	}
	// A synthetic shaper can choose a fixed advance of 500 font units.
	// The face itself supplies only metrics and scaling, not glyph advances.
	fmt.Println("units per em:", metrics.UnitsPerEm)
	fmt.Println("half-em advance in pt:", scale.ToDU(500)/float64(dimen.PT))
	fmt.Println("descent in pt:", scale.ToDU(metrics.Descent)/float64(dimen.PT))
	// Output:
	// units per em: 1000
	// half-em advance in pt: 5
	// descent in pt: -2
}
