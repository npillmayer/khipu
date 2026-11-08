package knuthplass

import (
	"fmt"
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
						params := NewKPDefaultParameters()
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
