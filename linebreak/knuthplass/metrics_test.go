package knuthplass

import (
	"reflect"
	"slices"
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
)

// Attach once before processing. Scanning here also supports focused fixtures
// which already contain several origins; production needs no enabling API.
func enableWorkMetrics(kp *linebreaker) {
	kp.metrics = &workMetrics{}
	for _, lines := range kp.paths {
		kp.metrics.live += len(lines)
	}
	kp.metrics.peak = kp.metrics.live
}

// Like the existing workload metrics, these describe the last independent pass,
// not sums over b.N. All benchmark inputs and policies are deterministic.
func reportWorkMetrics(b *testing.B, kp *linebreaker) {
	b.Helper()
	m := kp.metrics
	b.ReportMetric(float64(m.peak), "peak-active-states")
	b.ReportMetric(float64(m.ordinary), "ordinary-evals/op")
	b.ReportMetric(float64(m.discretionary), "discretionary-evals/op")
	b.ReportMetric(float64(m.ordinary+m.discretionary), "state-evals/op")
}

func TestWorkMetricsLifecycle(t *testing.T) {
	kp, err := prepareLineBreaker(DefaultLayout(linebreak.RectangularParShape(150*dimen.PT)), nil)
	if err != nil {
		t.Fatal(err)
	}
	if kp.metrics != nil {
		t.Fatal("production must default to no instrumentation")
	}
	enableWorkMetrics(kp)
	to := PlainBreak(1)
	kp.evalNewSegment(kp.root, to, 1, cost{demerits: 10})
	kp.evalNewSegment(kp.root, to, 1, cost{demerits: 20}) // losing replacement
	kp.evalNewSegment(kp.root, to, 1, cost{demerits: 5})  // winning replacement
	if kp.metrics.live != 2 || kp.metrics.peak != 2 {
		t.Fatalf("replacement changed state count: %+v", kp.metrics)
	}
	kp.evaluateCandidates(kp.root, 0)
	kp.evaluateCandidates(to, 0)
	kp.evaluateCandidates(PlainBreak(99), 0)
	if kp.metrics.ordinary != 2 {
		t.Fatalf("classifications=%d, want 2", kp.metrics.ordinary)
	}
	kp.retireState(origin{to, 1})
	kp.retireState(origin{to, 1}) // idempotent deletion
	if kp.evalNewSegment(kp.root, to, 1, cost{}) {
		t.Fatal("retired state revived")
	}
	if kp.metrics.live != 1 || kp.metrics.peak != 2 || len(kp.preds) != 1 {
		t.Fatalf("history/resurrection affected counts: %+v", kp.metrics)
	}
}

func TestWorkMetricsDiscretionaryClassifications(t *testing.T) {
	khp := newKhipu([]node{{w: 100, minw: 100, maxw: 100, p: InfinityDemerits, kind: khipu.KTTextBox}})
	for i, width := range []dimen.DU{100 * dimen.BP, 200 * dimen.BP} {
		khp.AddDiscretionaryCandidate(0, khipu.DiscretionaryCandidate{
			Variant:  uint16(i + 1),
			PreBreak: khipu.KnotCore{Kind: khipu.KTTextBox, W: width, MinW: width, MaxW: width},
		})
	}
	kp, _ := prepareLineBreaker(khipu.Params{LineShape: linebreak.RectangularParShape(100 * dimen.BP)}, nil)
	enableWorkMetrics(kp)
	for range 2 {
		if got := kp.evaluateDiscretionaryCandidates(khp, kp.root, -1, 0); len(got) != 1 {
			t.Fatalf("want one accepted and one rejected variant, got %d", len(got))
		}
	}
	kp.evaluateDiscretionaryLine(khp, kp.root, 99, -1, 0)    // filtered origin
	kp.evaluateDiscretionaryCandidates(khp, kp.root, -1, 99) // invalid candidate
	if kp.metrics.discretionary != 4 || kp.metrics.ordinary != 0 {
		t.Fatalf("counts=%+v", kp.metrics)
	}
}

func TestWorkMetricsDoNotChangeResults(t *testing.T) {
	var want []BreakRef
	var quality pathQuality
	var preds map[pathState]predecessor
	for _, measured := range []bool{false, true} {
		khp, kp, _, _ := hyphenatingLorem(t, 50)
		if measured {
			enableWorkMetrics(kp)
		}
		kp.end = PlainBreak(len(khp.Kind) - 1)
		kp.prepareRetirement(khp)
		for i := range khp.Kind {
			if live, err := kp.processKnot(khp, i); err != nil || !live {
				t.Fatalf("knot %d: live=%v err=%v", i, live, err)
			}
			if measured {
				live := 0
				for _, lines := range kp.paths {
					live += len(lines)
				}
				if kp.metrics.live != live || kp.metrics.peak < live {
					t.Fatalf("knot %d: actual live=%d metrics=%+v", i, live, kp.metrics)
				}
			}
		}
		got, q, ok := kp.collectOptimalBreakRefs(kp.end)
		if !ok {
			t.Fatal("no complete path")
		}
		if !measured {
			want, quality, preds = got, q, kp.preds
		} else if !slices.Equal(want, got) || quality != q || !reflect.DeepEqual(preds, kp.preds) {
			t.Fatal("instrumentation changed scoring or predecessor history")
		}
	}
}
