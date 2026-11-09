package knuthplass

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/schuko/tracing"
	"github.com/npillmayer/uax/bidi"
)

type emergencyWorkload struct {
	name, word, provider string
	fallback             bool
}

var emergencyWorkloads = []emergencyWorkload{
	{name: "loose-none", word: "ab"},
	{name: "loose-declining", word: "ab", provider: "empty"},
	{name: "split-productive", word: "abcd", provider: "productive"},
	{name: "overfull-declining", word: "abcd", provider: "empty", fallback: true},
}

type emergencyWorkloadFixture struct {
	khp        *khipu.Khipu
	layout     khipu.Params
	params     *Parameters
	provider   *countingLoremProvider
	empty      *decliningHyphenator
	productive *loremHyphenator
}

func (f emergencyWorkloadFixture) policyCalls() int { return f.empty.calls + f.productive.calls }

// Every ordinary word is 10 pt (ab) or 20 pt (abcd), and the target is 17 pt
// without side skips. Two short words cannot fit together. A short line needs
// E=6; an overlong word needs a 15/10 pt discretionary or final fallback.
// Repetition provides controlled scaling, not a model of linguistic text.
func newEmergencyWorkload(tb testing.TB, tc emergencyWorkload, words int) emergencyWorkloadFixture {
	tb.Helper()
	f := emergencyWorkloadFixture{empty: &decliningHyphenator{}, productive: &loremHyphenator{}}
	config := khipu.Config{}
	switch tc.provider {
	case "empty":
		config.Hyphenator = f.empty
	case "productive":
		config.Hyphenator = f.productive
	}
	owner := khipu.NewKhipukamayuq(config)
	text := styled.TextFromString(strings.Repeat(tc.word+" ", words))
	para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), bidi.LeftToRight, nil)
	if err != nil {
		tb.Fatal(err)
	}
	f.khp, err = owner.EncodeParagraph(tb.Context(), para)
	if err != nil {
		tb.Fatal(err)
	}
	f.provider = &countingLoremProvider{owner: owner}
	f.params = NewDefaultParameters()
	f.params.EmergencyStretch = 6 * dimen.PT
	if tc.provider != "" {
		f.params.DiscretionaryProvider = f.provider
	}
	f.layout = khipu.Params{LineShape: linebreak.RectangularParShape(17 * dimen.PT)}
	if len(f.khp.Kind) != words*2+1 || f.policyCalls() != 0 || len(f.khp.DiscretionaryCandidates) != 0 {
		tb.Fatal("fixture must start with ordinary knots and cold discovery")
	}
	return f
}

type emergencyPassWork struct {
	work            workMetrics
	requests, calls int
}

func (w *emergencyPassWork) add(p emergencyPassWork) {
	// Passes run serially: peaks are maximized, not summed. live is a
	// per-pass ending value and intentionally has no whole-call aggregate.
	w.work.peak = max(w.work.peak, p.work.peak)
	w.work.ordinary += p.work.ordinary
	w.work.ordinaryReused += p.work.ordinaryReused
	w.work.discretionary += p.work.discretionary
	w.requests += p.requests
	w.calls += p.calls
}

type emergencyDiagnostic struct {
	result     passResult
	passes     [3]emergencyPassWork
	total      emergencyPassWork
	selections map[int]khipu.DiscretionarySelection
	fallbacks  int
}

// This untimed diagnostic uses production pass configurations and pass bodies,
// enabling existing private counters before each scan. It deliberately does not
// add a hook to the public API. Tests and each timed benchmark iteration verify
// results and provider/policy counts against the uninstrumented entry points.
func diagnoseEmergencyWorkload(tb testing.TB, tc emergencyWorkload, words int) emergencyDiagnostic {
	tb.Helper()
	f := newEmergencyWorkload(tb, tc, words)
	passes, err := paragraphPasses(f.params)
	if err != nil || len(passes) != 3 {
		tb.Fatal("workload must enable three passes", err)
	}
	var d emergencyDiagnostic
	for i, pass := range passes {
		kp, err := prepareLineBreaker(f.layout, f.params)
		if err != nil {
			tb.Fatal(err)
		}
		kp.hyphenating, kp.final, kp.emergencyStretch = pass.hyphenating, pass.final, pass.emergencyStretch
		enableWorkMetrics(kp)
		requests, calls := f.provider.requests, f.policyCalls()
		if err := kp.buildBreakpointPaths(f.khp); err != nil {
			tb.Fatal(err)
		}
		refs, quality, complete := kp.collectOptimalBreakRefs(kp.end)
		d.passes[i] = emergencyPassWork{work: *kp.metrics, requests: f.provider.requests - requests, calls: f.policyCalls() - calls}
		d.total.add(d.passes[i])
		if i < 2 {
			if complete || len(refs) != 0 {
				tb.Fatalf("%s unexpectedly completed in pass %d", tc.name, pass.id)
			}
			continue
		}
		if !complete || len(refs) == 0 || refs[len(refs)-1] != kp.end {
			tb.Fatal("emergency pass failed to complete", refs)
		}
		d.result = passResult{pass: pass, breaks: refs, quality: quality, complete: complete}
		// These fixtures have bounded small accepted costs, distinct from the
		// canonical fallback cost. Inspect chosen transitions, not total cost.
		for line, to := range refs {
			pred, ok := kp.Pred(to, lineNo(line+1))
			if !ok {
				tb.Fatal("missing chosen predecessor", to)
			}
			if pred.cost == AwfulDemerits+kp.terminalHyphenDemerits(pred.from, to) {
				d.fallbacks++
			}
		}
	}
	recordSelectedDiscretionaries(f.khp, d.result.breaks)
	d.selections = maps.Clone(f.khp.SelectedDiscretionaries)
	if tc.fallback != (d.fallbacks > 0) || !tc.fallback && d.result.quality.worstBadness > f.params.Tolerance {
		tb.Fatal("wrong recovery/fallback workload", d.result.quality, d.fallbacks)
	}
	if tc.provider == "productive" && (len(d.selections) != words || d.total.work.discretionary == 0 || d.total.calls == 0) {
		tb.Fatal("productive workload must select one shaped variant per word")
	}
	if tc.provider == "empty" && (d.total.calls == 0 || d.total.requests <= d.total.calls || d.total.work.discretionary != 0 || len(d.selections) != 0) {
		tb.Fatal("declining workload must exercise cached empty requests")
	}
	return d
}

// Preserve exactly the discovery state reached by the real first two passes;
// isolated timing must not start with an artificially cold or fully warm cache.
func warmEmergencyPrelude(tb testing.TB, f emergencyWorkloadFixture) passConfig {
	tb.Helper()
	passes, err := paragraphPasses(f.params)
	if err != nil || len(passes) != 3 {
		tb.Fatal(err)
	}
	for _, pass := range passes[:2] {
		result, err := breakParagraphPassRefs(f.khp, f.layout, f.params, pass)
		if err != nil || result.complete {
			tb.Fatal("prelude must fail normally", result, err)
		}
	}
	return passes[2]
}

func reportEmergencyWork(b *testing.B, d emergencyDiagnostic, isolated bool) {
	w, passes := d.total, 3
	if isolated {
		w, passes = d.passes[2], 1
	}
	b.ReportMetric(float64(passes), "passes/op") // timed passes, not untimed prelude
	b.ReportMetric(float64(d.result.pass.id), "selected-pass")
	b.ReportMetric(float64(w.work.peak), "peak-active-states")
	b.ReportMetric(float64(w.work.ordinary), "ordinary-evals/op")
	b.ReportMetric(float64(w.work.ordinaryReused), "ordinary-reused/op")
	b.ReportMetric(float64(w.work.discretionary), "discretionary-evals/op")
	b.ReportMetric(float64(w.work.ordinary+w.work.discretionary), "state-evals/op")
	b.ReportMetric(float64(w.requests), "provider-requests/op")
	b.ReportMetric(float64(w.calls), "hyphenator-calls/op")
	b.ReportMetric(float64(len(d.result.breaks)), "lines")
	b.ReportMetric(float64(d.fallbacks), "fallback-lines")
	b.ReportMetric(float64(d.result.quality.worstBadness), "worst-badness")
	if !isolated {
		for i, pass := range d.passes {
			b.ReportMetric(float64(pass.work.ordinary+pass.work.discretionary), fmt.Sprintf("p%d-state-evals/op", i+1))
		}
	}
}

// BenchmarkEmergencyWorkloads times the uninstrumented public call or isolated
// pass 3 (including its setup and collection). Both use a fresh owner/encoding
// per iteration, outside timing. Public timing includes validation, all three
// passes, lazy shaping and publication. Isolated timing excludes the first two
// passes and publication, but includes new pass-3 discovery. Work counters are
// from a separate, verified diagnostic; they do not affect time or allocs/op.
func BenchmarkEmergencyWorkloads(b *testing.B) {
	if tracer().GetTraceLevel() >= tracing.LevelInfo {
		b.Fatal("benchmark requires info/debug logging disabled (use -run '^$')")
	}
	for _, words := range []int{16, 128, 512} {
		for _, tc := range emergencyWorkloads {
			for _, scope := range []string{"public", "isolated"} {
				b.Run(fmt.Sprintf("%d/%s/%s", words, tc.name, scope), func(b *testing.B) {
					b.StopTimer()
					d := diagnoseEmergencyWorkload(b, tc, words)
					isolated := scope == "isolated"
					b.ReportAllocs()
					b.ResetTimer()
					for range b.N {
						f := newEmergencyWorkload(b, tc, words)
						var pass passConfig
						if isolated {
							pass = warmEmergencyPrelude(b, f)
						}
						requests, calls := f.provider.requests, f.policyCalls()
						var breaks []kinx
						var result passResult
						var err error
						b.StartTimer()
						if isolated {
							result, err = breakParagraphPassRefs(f.khp, f.layout, f.params, pass)
						} else {
							breaks, err = BreakParagraph(f.khp, f.layout, f.params)
						}
						b.StopTimer()
						if err != nil {
							b.Fatal(err)
						}
						want := d.total
						if isolated {
							want = d.passes[2]
							if !result.complete || result.pass != d.result.pass || result.quality != d.result.quality || !slices.Equal(result.breaks, d.result.breaks) {
								b.Fatal("isolated result differs from diagnostic")
							}
						} else if !slices.Equal(breaks, projectBreakRefs(d.result.breaks)) || !maps.Equal(f.khp.SelectedDiscretionaries, d.selections) {
							b.Fatal("public result differs from diagnostic")
						}
						if f.provider.requests-requests != want.requests || f.policyCalls()-calls != want.calls {
							b.Fatal("timed provider work differs from diagnostic")
						}
					}
					reportEmergencyWork(b, d, isolated)
				})
			}
		}
	}
}

// Fixture assertions run in the normal suite too: benchmarks must never
// silently turn into first-pass successes after an unrelated algorithm change.
func TestEmergencyBenchmarkWorkloads(t *testing.T) {
	for _, tc := range emergencyWorkloads {
		t.Run(tc.name, func(t *testing.T) {
			d := diagnoseEmergencyWorkload(t, tc, 16)
			f := newEmergencyWorkload(t, tc, 16)
			result, err := selectParagraphBreaks(f.khp, f.layout, f.params)
			if err != nil || !result.complete || result.pass != d.result.pass || result.quality != d.result.quality || !slices.Equal(result.breaks, d.result.breaks) ||
				f.provider.requests != d.total.requests || f.policyCalls() != d.total.calls {
				t.Fatal("instrumented search differs from production coordinator", result, err)
			}
			for i, p := range d.passes {
				t.Logf("pass %d: peak=%d ordinary=%d reused=%d discretionary=%d provider=%d policy=%d",
					i+1, p.work.peak, p.work.ordinary, p.work.ordinaryReused, p.work.discretionary, p.requests, p.calls)
			}
		})
	}
}
