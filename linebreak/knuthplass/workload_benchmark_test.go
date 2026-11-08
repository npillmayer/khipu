package knuthplass

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/schuko/tracing"
)

type decliningHyphenator struct{ calls int }

func (h *decliningHyphenator) Discretionaries(string, *khipu.Params) ([]khipu.HyphenationFragment, error) {
	h.calls++
	return nil, nil
}

// Work baselines originated before empty-result caching; quality is updated for
// neutral Unicode opportunities. Policy calls may fall, but discovery requests,
// scoring demands, history and final decisions must not change across warm runs.
// Boundary reuse may now satisfy an ordinary demand without a classification.
func TestEmptyDiscoveryCachePreservesPassWork(t *testing.T) {
	for _, tc := range []struct {
		mode                         string
		ordinary, discretionary      uint64
		peak, predecessors, requests int
		quality                      pathQuality
	}{
		{"empty", 1230, 0, 33, 129, 390, pathQuality{6481, 43}},
		{"productive", 2828, 1695, 74, 284, 929, pathQuality{1855, 3}},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			empty, productive := &decliningHyphenator{}, &loremHyphenator{}
			config := khipu.Config{Hyphenator: empty}
			if tc.mode == "productive" {
				config.Hyphenator = productive
			}
			owner := khipu.NewKhipukamayuq(config)
			khp, err := owner.EncodeParagraph(t.Context(), loremParagraph(t, 50))
			if err != nil {
				t.Fatal(err)
			}
			provider := &countingLoremProvider{owner: owner}
			params := NewDefaultParameters()
			params.DiscretionaryProvider = provider
			var first []BreakRef
			var preds map[pathState]predecessor
			calls := 0
			for run := range 2 {
				kp, err := prepareLineBreaker(DefaultLayout(linebreak.RectangularParShape(150*dimen.PT)), params)
				if err != nil {
					t.Fatal(err)
				}
				kp.final, kp.hyphenating = true, true
				enableWorkMetrics(kp)
				if err := kp.constructBreakpointGraph(khp); err != nil {
					t.Fatal(err)
				}
				breaks, quality, ok := kp.collectOptimalBreakRefs(kp.end)
				if !ok || len(breaks) != 12 || quality != tc.quality {
					t.Fatalf("changed best path: %v %+v", breaks, quality)
				}
				if run == 0 {
					if kp.metrics.ordinary+kp.metrics.ordinaryReused != tc.ordinary || kp.metrics.discretionary != tc.discretionary || kp.metrics.peak != tc.peak || len(kp.preds) != tc.predecessors || provider.requests != tc.requests {
						t.Fatalf("cold-pass work changed: %+v preds=%d requests=%d", kp.metrics, len(kp.preds), provider.requests)
					}
					first, preds = breaks, kp.preds
					calls = empty.calls + productive.calls
					if calls == 0 || calls > 50 {
						t.Fatalf("expected at most one policy call per textbox, got %d", calls)
					}
				} else if empty.calls+productive.calls != calls || !reflect.DeepEqual(first, breaks) || !reflect.DeepEqual(preds, kp.preds) {
					t.Fatal("warm paragraph queried policy again or changed paths/history")
				}
			}
		})
	}
}

// BenchmarkRetirementWorkloads compares one final pass for every case, not the
// public two-pass call. Ordinary disables discovery; empty/productive enable it.
// All cases permit fallback so narrow/forced layouts measure completed work.
// Ordinary uses Tolerance too, despite disabling discovery, to keep geometric
// admission comparable; the hyphenating cases retain the default PreTolerance
// discovery trigger. The separate first-pass benchmark keeps production defaults.
// Setup is untimed and cold per iteration, including the provider's candidate
// cache. Forced cases mark every twentieth interword space as a mandatory break;
// they deliberately retain ordinary space geometry rather than adding fill.
func BenchmarkRetirementWorkloads(b *testing.B) {
	if tracer().GetTraceLevel() >= tracing.LevelInfo {
		b.Fatal("benchmark requires info/debug logging disabled (use -run '^$')")
	}
	for _, words := range []int{50, 250, 1000} {
		for _, geometry := range []struct {
			name   string
			shape  khipu.LineShape
			forced bool
		}{
			{"narrow", linebreak.RectangularParShape(100 * dimen.PT), false},
			{"standard", linebreak.RectangularParShape(150 * dimen.PT), false},
			{"wide", linebreak.RectangularParShape(300 * dimen.PT), false},
			{"alternating", alternatingIndexShape{}, false},
			{"forced", linebreak.RectangularParShape(150 * dimen.PT), true},
		} {
			for _, mode := range []string{"ordinary", "empty", "productive"} {
				b.Run(fmt.Sprintf("%d/%s/%s", words, geometry.name, mode), func(b *testing.B) {
					b.ReportAllocs()
					b.StopTimer()
					for range b.N {
						productive, empty := &loremHyphenator{}, &decliningHyphenator{}
						config := khipu.Config{}
						if mode == "productive" {
							config.Hyphenator = productive
						} else if mode == "empty" {
							config.Hyphenator = empty
						}
						owner := khipu.NewKhipukamayuq(config)
						khp, err := owner.EncodeParagraph(b.Context(), loremParagraph(b, words))
						if err != nil {
							b.Fatal(err)
						}
						if geometry.forced {
							for at := 39; at < len(khp.Kind)-1; at += 40 {
								khp.Penalty[at] = InfinityMerits
							}
						}
						provider := &countingLoremProvider{owner: owner}
						params := NewDefaultParameters()
						if mode != "ordinary" {
							params.DiscretionaryProvider = provider
						} else {
							params.PreTolerance = params.Tolerance
						}
						kp, err := prepareLineBreaker(DefaultLayout(geometry.shape), params)
						if err != nil {
							b.Fatal(err)
						}
						kp.final, kp.hyphenating = true, mode != "ordinary"
						enableWorkMetrics(kp)
						b.StartTimer()
						err = kp.constructBreakpointGraph(khp)
						breaks, quality, ok := kp.collectOptimalBreakRefs(kp.end)
						b.StopTimer()
						if err != nil || !ok || len(breaks) == 0 || breaks[len(breaks)-1] != kp.end {
							b.Fatalf("incomplete path: %v, %v", breaks, err)
						}
						if mode == "empty" && (empty.calls == 0 || kp.metrics.discretionary != 0) {
							b.Fatal("empty provider workload did not decline discovery")
						}
						if mode == "productive" && (productive.calls == 0 || kp.metrics.discretionary == 0) {
							b.Fatal("productive workload did not score variants")
						}
						if geometry.forced {
							selected := make(map[BreakRef]bool, len(breaks))
							for _, bp := range breaks {
								selected[bp] = true
							}
							for at := 39; at < len(khp.Kind)-1; at += 40 {
								if !selected[PlainBreak(at)] {
									b.Fatalf("skipped forced boundary %d", at)
								}
							}
						}
						reportWorkMetrics(b, kp)
						b.ReportMetric(float64(provider.requests), "provider-requests/op")
						b.ReportMetric(float64(productive.calls+empty.calls), "hyphenator-calls/op")
						b.ReportMetric(float64(kp.metrics.live), "retained-states")
						b.ReportMetric(float64(len(kp.preds)), "predecessors")
						b.ReportMetric(float64(len(breaks)), "lines")
						b.ReportMetric(float64(quality.totalCost), "demerits")
						b.ReportMetric(float64(quality.worstBadness), "worst-badness")
					}
				})
			}
		}
	}
}
