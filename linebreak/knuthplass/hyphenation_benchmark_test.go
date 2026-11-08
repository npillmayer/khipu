package knuthplass

import (
	"fmt"
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/schuko/tracing"
)

// loremHyphenator is a workload policy, not linguistic Latin hyphenation. One
// midpoint alternative per eligible ASCII word bounds variant fan-out while
// exercising real fragment shaping, caching and tight-line seed rescans.
type loremHyphenator struct{ calls int }

func (h *loremHyphenator) Discretionaries(word string, _ *khipu.Params) ([]khipu.HyphenationFragment, error) {
	h.calls++
	if len(word) < 4 {
		return nil, nil
	}
	mid := len(word) / 2
	return []khipu.HyphenationFragment{{
		PreBreak: word[:mid] + "-", PostBreak: word[mid:],
		SourceSplit: khipu.SourceSplit{ByteOffset: uint64(mid), Valid: true},
	}}, nil
}

// Count provider requests separately from policy calls: cached requests avoid
// shaping, but still lead to discretionary scoring in the linebreaker.
type countingLoremProvider struct {
	owner    *khipu.Khipukamayuq
	requests int
}

func (p *countingLoremProvider) DiscretionaryCandidates(khp *khipu.Khipu, at int) ([]khipu.DiscretionaryCandidate, error) {
	p.requests++
	return p.owner.DiscretionaryCandidates(khp, at)
}

func hyphenatingLorem(tb testing.TB, words int) (*khipu.Khipu, *linebreaker, *countingLoremProvider, *loremHyphenator) {
	tb.Helper()
	policy := &loremHyphenator{}
	owner := khipu.NewKhipukamayuq(khipu.Config{Hyphenator: policy})
	khp, err := owner.EncodeParagraph(tb.Context(), loremParagraph(tb, words))
	if err != nil {
		tb.Fatal(err)
	}
	if len(khp.Kind) != 2*words+1 || policy.calls != 0 {
		tb.Fatal("fixture must have ordinary lorem knots and no eager hyphenation")
	}
	provider := &countingLoremProvider{owner: owner}
	params := NewDefaultParameters()
	params.DiscretionaryProvider = provider
	kp, err := prepareLineBreaker(DefaultLayout(linebreak.RectangularParShape(150*dimen.PT)), params)
	if err != nil {
		tb.Fatal(err)
	}
	// Default lorem can finish in pass 1. Explicitly select pass 2 rather than
	// changing its thresholds or claiming to measure the public two-pass call.
	kp.hyphenating = true
	kp.final = true
	return khp, kp, provider, policy
}

// BenchmarkSecondPassHyphenatingLorem includes lazy discovery, fragment shaping,
// graph construction and optimal-path collection, but not paragraph encoding or
// breaker setup. Every iteration starts with a fresh owner/Khipu: warming the
// discretionary cache across iterations would hide part of the workload.
// Smaller sizes than the ordinary benchmark keep the rescan-heavy case usable.
func BenchmarkSecondPassHyphenatingLorem(b *testing.B) {
	if tracer().GetTraceLevel() >= tracing.LevelInfo {
		b.Fatal("benchmark requires info/debug logging disabled (use -run '^$')")
	}
	for _, words := range []int{50, 100, 250} {
		b.Run(fmt.Sprint(words), func(b *testing.B) {
			b.ReportAllocs()
			b.StopTimer()
			for range b.N {
				khp, kp, provider, policy := hyphenatingLorem(b, words)
				enableWorkMetrics(kp)
				b.StartTimer()
				err := kp.constructBreakpointGraph(khp)
				breaks, quality, ok := kp.collectOptimalBreakRefs(kp.end)
				b.StopTimer()
				reportWorkMetrics(b, kp)
				if err != nil || !ok || len(breaks) == 0 || breaks[len(breaks)-1] != kp.end {
					b.Fatalf("incomplete second-pass result: %v, %v", breaks, err)
				}
				retained, discretionary := 0, 0
				for _, lines := range kp.paths {
					retained += len(lines)
				}
				for state := range kp.preds {
					if state.to.IsDiscretionary() {
						discretionary++
					}
				}
				if policy.calls == 0 || provider.requests <= policy.calls || discretionary == 0 {
					b.Fatal("fixture did not exercise discovery, cached requests and discretionary paths")
				}
				// Deterministic workload counts from the last iteration, not sums
				// over b.N. Retained totals are separate from peak-active-states.
				b.ReportMetric(float64(len(khp.Kind)), "knots")
				b.ReportMetric(float64(provider.requests), "provider-requests/op")
				b.ReportMetric(float64(policy.calls), "hyphenator-calls/op")
				b.ReportMetric(float64(retained), "retained-states")
				b.ReportMetric(float64(discretionary), "discretionary-predecessors")
				b.ReportMetric(float64(len(breaks)), "lines")
				b.ReportMetric(float64(quality.totalCost), "demerits")
				b.ReportMetric(float64(quality.worstBadness), "worst-badness")
			}
		})
	}
}

func TestHyphenatingLoremExercisesColdDiscoveryAndPaths(t *testing.T) {
	khp, kp, provider, policy := hyphenatingLorem(t, 25)
	if err := kp.constructBreakpointGraph(khp); err != nil {
		t.Fatal(err)
	}
	breaks, _, ok := kp.collectOptimalBreakRefs(kp.end)
	if !ok || len(breaks) == 0 || breaks[len(breaks)-1] != kp.end {
		t.Fatal("incomplete second-pass result")
	}
	if policy.calls == 0 || provider.requests <= policy.calls {
		t.Fatal("fixture did not exercise discovery and cached requests")
	}
	for state := range kp.preds {
		if state.to.IsDiscretionary() {
			return
		}
	}
	t.Fatal("fixture did not create discretionary paths")
}

func TestHyphenatingLoremTightStateRequestsCachedCandidates(t *testing.T) {
	khp, kp, provider, policy := hyphenatingLorem(t, 25)
	// Keep one origin and advance far beyond a feasible line. Positive shrink
	// capacity still gives a computable ratio, so the capped badness passes the
	// hyphenation gate even though shortening its last word cannot save the line.
	const breakpoint = 39 // Space after word 20 ("lorem").
	for i := 0; i <= breakpoint; i++ {
		kp.updatePath(kp.root, i, khp.KnotByIndex(i))
	}
	evals := kp.evaluateCandidates(kp.root, khp.Penalty[breakpoint])
	if len(evals) != 1 {
		t.Fatalf("got %d evaluations", len(evals))
	}
	ev := evals[0]
	if ev.badness != 10000 || ev.ratio >= 0 || ev.accepted() || !kp.wantsHyphenation(ev) {
		t.Fatalf("fixture did not produce a screened-out tight state: %+v", ev)
	}
	for range 2 {
		if err := kp.requestPass2Discretionaries(khp, breakpoint, ev); err != nil {
			t.Fatal(err)
		}
		candidate, ok := hyphenationCandidateForPass2(khp, breakpoint, ev)
		if !ok || candidate != breakpoint-1 || len(khp.DiscretionariesAt(candidate)) != 1 {
			t.Fatal("expected a cached alternative behind the current breakpoint")
		}
		// candidate <= breakpoint exercises replay from the stored seed, not
		// lookahead. This test intentionally documents the current wasted work.
		if got := kp.evaluateDiscretionaryCandidates(khp, kp.root, breakpoint, candidate); len(got) != 0 {
			t.Fatal("splitting the last word unexpectedly rescued the overlong line")
		}
	}
	if provider.requests != 2 || policy.calls != 1 {
		t.Fatalf("requests=%d policy calls=%d; want cached second request", provider.requests, policy.calls)
	}
}
