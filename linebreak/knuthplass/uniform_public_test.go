package knuthplass

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/schuko/tracing"
)

// Wrapping the same immutable shape hides its concrete type, exercising the
// production baseline without adding a public optimization switch or changing
// any width. Both arms call the real public multi-pass entry point.
type opaqueUniformShape struct{ khipu.LineShape }

func uniformPublicFixture(tb testing.TB, name string, words int, compact bool) (*khipu.Khipu, khipu.Params, *Parameters, passID) {
	tb.Helper()
	var khp *khipu.Khipu
	var layout khipu.Params
	var params *Parameters
	wantPass := passPreTolerance
	if name == "first" || name == "second-productive" {
		config := khipu.Config{}
		if name == "second-productive" {
			config.Hyphenator = &loremHyphenator{}
		}
		owner := khipu.NewKhipukamayuq(config)
		var err error
		khp, err = owner.EncodeParagraph(tb.Context(), loremParagraph(tb, words))
		if err != nil {
			tb.Fatal(err)
		}
		layout = DefaultLayout(linebreak.RectangularParShape(150 * dimen.PT))
		// Keep nil parameters on the first-pass fixture to cover the public default.
		if name == "second-productive" {
			params = NewDefaultParameters()
			params.PreTolerance = 0
			params.DiscretionaryProvider = owner
			wantPass = passTolerance
		}
	} else {
		found := false
		for _, workload := range emergencyWorkloads {
			if workload.name == name {
				f := newEmergencyWorkload(tb, workload, words)
				khp, layout, params = f.khp, f.layout, f.params
				found = true
				break
			}
		}
		if !found {
			tb.Fatalf("unknown fixture %q", name)
		}
		wantPass = passEmergency
	}
	if !compact {
		layout.LineShape = opaqueUniformShape{layout.LineShape}
	}
	return khp, layout, params, wantPass
}

func TestUniformPublicPasses(t *testing.T) {
	for _, name := range []string{"first", "second-productive", "loose-none", "loose-declining", "split-productive", "overfull-declining"} {
		t.Run(name, func(t *testing.T) {
			var baseline passResult
			var selected map[int]khipu.DiscretionarySelection
			for _, compact := range []bool{false, true} {
				khp, layout, params, wantPass := uniformPublicFixture(t, name, 50, compact)
				if newLinebreaker(layout, params).compactUniform != compact {
					t.Fatal("incorrect automatic dispatch")
				}
				// Cold public call and warm retry must return the same break/variant
				// decisions. Selection publication is part of the comparison.
				for range 2 {
					breaks, err := BreakParagraph(khp, layout, params)
					if err != nil {
						t.Fatal(err)
					}
					result, err := selectParagraphBreaks(khp, layout, params)
					if err != nil || result.pass.id != wantPass || !result.complete || !slices.Equal(breaks, projectBreakRefs(result.breaks)) {
						t.Fatalf("wrong pass or public result: %+v %v", result, err)
					}
					if !baseline.complete {
						baseline, selected = result, maps.Clone(khp.SelectedDiscretionaries)
					} else if !reflect.DeepEqual(result, baseline) || !maps.Equal(khp.SelectedDiscretionaries, selected) {
						t.Fatalf("compaction or warm retry changed results: %+v vs %+v", result, baseline)
					}
				}
				if (name == "second-productive" || name == "split-productive") && len(selected) == 0 {
					t.Fatal("fixture selected no variants")
				}
			}
		})
	}
}

// Complete public calls: pass dispatch, fresh pass setup, lazy provider/shaping
// work and final publication are timed. Paragraph encoding is excluded and the
// owner/candidate cache is cold for every iteration. Optional workMetrics
// instrumentation is disabled, as in normal public calls.
func BenchmarkUniformPublic(b *testing.B) {
	if tracer().GetTraceLevel() >= tracing.LevelInfo {
		b.Fatal("run with -run '^$'")
	}
	for _, words := range []int{250, 1000} {
		for _, name := range []string{"first", "second-productive", "split-productive", "overfull-declining"} {
			// Establish expected decisions outside the timed region, including
			// which pass completes. Both benchmark arms check against baseline.
			khp, layout, params, wantPass := uniformPublicFixture(b, name, words, false)
			want, err := selectParagraphBreaks(khp, layout, params)
			if err != nil || !want.complete || want.pass.id != wantPass {
				b.Fatalf("wrong benchmark pass: %+v %v", want, err)
			}
			recordSelectedDiscretionaries(khp, want.breaks)
			selected := maps.Clone(khp.SelectedDiscretionaries)
			for _, compact := range []bool{false, true} {
				b.Run(fmt.Sprintf("%d/%s/compact%v", words, name, compact), func(b *testing.B) {
					b.ReportAllocs()
					b.StopTimer()
					for range b.N {
						khp, layout, params, _ := uniformPublicFixture(b, name, words, compact)
						b.StartTimer()
						got, err := BreakParagraph(khp, layout, params)
						b.StopTimer()
						if err != nil || !slices.Equal(got, projectBreakRefs(want.breaks)) || !maps.Equal(khp.SelectedDiscretionaries, selected) {
							b.Fatal("public result differs from baseline", err)
						}
					}
					b.ReportMetric(float64(len(want.breaks)), "lines/op")
				})
			}
		}
	}
}
