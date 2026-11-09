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

type unadvertisedUniformShape struct{}

func (unadvertisedUniformShape) LineLength(int32) dimen.DU { return 150 * dimen.PT }

func uniformFixture(tb testing.TB, words int, shape khipu.LineShape, mode string, pass passConfig, forced bool, reward bool) (*linebreaker, *khipu.Khipu) {
	tb.Helper()
	config := khipu.Config{}
	switch mode {
	case "empty":
		config.Hyphenator = &decliningHyphenator{}
	case "productive":
		config.Hyphenator = &loremHyphenator{}
	}
	owner := khipu.NewKhipukamayuq(config)
	khp, err := owner.EncodeParagraph(tb.Context(), loremParagraph(tb, words))
	if err != nil {
		tb.Fatal(err)
	}
	if reward {
		for i, p := range khp.Penalty {
			if p == 0 {
				khp.Penalty[i] = -29
			}
		}
	}
	if forced {
		for at := 39; at < len(khp.Kind)-1; at += 40 {
			khp.Penalty[at] = InfinityMerits
		}
	}
	params := NewDefaultParameters()
	params.FirstHyphenDemerits, params.FinalHyphenDemerits = 75, 125
	if mode != "ordinary" {
		params.DiscretionaryProvider = owner
	}
	kp, err := prepareLineBreaker(DefaultLayout(shape), params)
	if err != nil {
		tb.Fatal(err)
	}
	kp.hyphenating, kp.final, kp.emergencyStretch = pass.hyphenating, pass.final, pass.emergencyStretch
	return kp, khp
}

func TestUniformCompactionEquivalence(t *testing.T) {
	for _, shape := range []khipu.LineShape{linebreak.RectangularParShape(100 * dimen.PT), linebreak.RectangularParShape(150 * dimen.PT), alternatingIndexShape{}, unadvertisedUniformShape{}} {
		for _, mode := range []string{"ordinary", "empty", "productive"} {
			for _, pass := range []passConfig{{id: passPreTolerance}, {id: passTolerance, hyphenating: true, final: true}, {id: passEmergency, hyphenating: true, final: true, emergencyStretch: 25 * dimen.BP}} {
				for _, forced := range []bool{false, true} {
					for _, reward := range []bool{false, true} {
						t.Run(fmt.Sprintf("%T/%v/%s/pass%d/forced%v/reward%v", shape, shape.LineLength(1), mode, pass.id, forced, reward), func(t *testing.T) {
							var want []BreakRef
							var quality pathQuality
							var complete bool
							for _, compact := range []bool{false, true, true} {
								kp, khp := uniformFixture(t, 50, shape, mode, pass, forced, reward)
								if compact {
									enabled := kp.compactUniform
									_, alternating := shape.(alternatingIndexShape)
									_, custom := shape.(unadvertisedUniformShape)
									if enabled == (alternating || custom) {
										t.Fatal("incorrect shape recognition")
									}
								}
								kp.compactUniform = compact && kp.compactUniform
								if err := kp.buildBreakpointPaths(khp); err != nil {
									t.Fatal(err)
								}
								got, q, ok := kp.collectOptimalBreakRefs(kp.end)
								if !compact {
									want, quality, complete = got, q, ok
								} else if !reflect.DeepEqual(got, want) || q != quality || ok != complete {
									t.Fatalf("compact changed result: %v %+v %v; baseline %v %+v %v", got, q, ok, want, quality, complete)
								}
								if !kp.compactUniform && kp.uniformRemoved != 0 {
									t.Fatal("unknown shape compacted")
								}
							}
						})
					}
				}
			}
		}
	}
}

func TestUniformCompactionPreservesHistoryAndTies(t *testing.T) {
	kp := newLinebreaker(DefaultLayout(linebreak.RectangularParShape(100)), nil)
	to := PlainBreak(3)
	kp.paths[to] = map[lineNo]bookkeeping{2: {totalcost: 10}, 3: {totalcost: 10}, 4: {totalcost: 20}}
	for line := range kp.paths[to] {
		kp.SetPred(PlainBreak(1), to, 10, kp.paths[to][line].totalcost, line)
	}
	kp.horizon[to] = struct{}{}
	kp.compactUniformBoundary(to)
	if len(kp.paths[to]) != 1 || kp.paths[to][2].totalcost != 10 || len(kp.preds) != 3 || kp.uniformRemoved != 2 {
		t.Fatal("wrong winner or lost history")
	}
	disc := BreakRef{At: 4, Variant: 1}
	kp.paths[disc] = map[lineNo]bookkeeping{2: {}, 3: {}}
	kp.compactUniformBoundary(disc)
	if len(kp.paths[disc]) != 2 {
		t.Fatal("compacted rediscoverable discretionary")
	}
	kp.paths[kp.root] = map[lineNo]bookkeeping{0: {}, 1: {}}
	kp.compactUniformBoundary(kp.root)
	if len(kp.paths[kp.root]) != 2 {
		t.Fatal("merged first-line state")
	}
}

func TestUniformCompactionLongParagraph(t *testing.T) {
	for _, mode := range []string{"ordinary", "productive"} {
		pass := passConfig{id: passPreTolerance}
		if mode == "productive" {
			pass = passConfig{id: passTolerance, hyphenating: true, final: true}
		}
		var want []BreakRef
		var quality pathQuality
		for _, compact := range []bool{false, true} {
			kp, khp := uniformFixture(t, 1000, linebreak.RectangularParShape(150*dimen.PT), mode, pass, false, false)
			kp.compactUniform = compact && kp.compactUniform
			if err := kp.buildBreakpointPaths(khp); err != nil {
				t.Fatal(err)
			}
			got, q, ok := kp.collectOptimalBreakRefs(kp.end)
			if !ok {
				t.Fatal("incomplete long fixture")
			}
			if !compact {
				want, quality = got, q
			} else if !reflect.DeepEqual(got, want) || q != quality || kp.uniformRemoved == 0 {
				t.Fatalf("%s: compaction changed long path/quality or did no work", mode)
			}
		}
	}
}

func TestUniformCompactionEmergencyAndFallback(t *testing.T) {
	for _, workload := range emergencyWorkloads {
		for _, pass := range []passConfig{{id: passPreTolerance}, {id: passTolerance, hyphenating: true}, {id: passEmergency, hyphenating: true, final: true, emergencyStretch: 6 * dimen.PT}} {
			var want []BreakRef
			var quality pathQuality
			var complete bool
			for _, compact := range []bool{false, true} {
				fixture := newEmergencyWorkload(t, workload, 16)
				kp, err := prepareLineBreaker(fixture.layout, fixture.params)
				if err != nil {
					t.Fatal(err)
				}
				kp.hyphenating, kp.final, kp.emergencyStretch = pass.hyphenating, pass.final, pass.emergencyStretch
				kp.compactUniform = compact && kp.compactUniform
				if err := kp.buildBreakpointPaths(fixture.khp); err != nil {
					t.Fatal(err)
				}
				got, q, ok := kp.collectOptimalBreakRefs(kp.end)
				if !compact {
					want, quality, complete = got, q, ok
				} else if !reflect.DeepEqual(got, want) || q != quality || ok != complete {
					t.Fatalf("%s/pass%d changed: %v %+v %v", workload.name, pass.id, got, q, ok)
				}
			}
		}
	}
}

func BenchmarkUniformCompaction(b *testing.B) {
	if tracer().GetTraceLevel() >= tracing.LevelInfo {
		b.Fatal("run with -run '^$'")
	}
	for _, words := range []int{250, 1000} {
		for _, mode := range []string{"ordinary", "productive"} {
			for _, compact := range []bool{false, true} {
				b.Run(fmt.Sprintf("%d/%s/compact%v", words, mode, compact), func(b *testing.B) {
					b.ReportAllocs()
					b.StopTimer()
					for range b.N {
						pass := passConfig{id: passPreTolerance}
						if mode != "ordinary" {
							pass = passConfig{id: passTolerance, hyphenating: true, final: true}
						}
						kp, khp := uniformFixture(b, words, linebreak.RectangularParShape(150*dimen.PT), mode, pass, false, false)
						kp.compactUniform = compact && kp.compactUniform
						enableWorkMetrics(kp)
						b.StartTimer()
						err := kp.buildBreakpointPaths(khp)
						breaks, q, ok := kp.collectOptimalBreakRefs(kp.end)
						b.StopTimer()
						if err != nil || !ok {
							b.Fatalf("incomplete: %v", err)
						}
						reportWorkMetrics(b, kp)
						b.ReportMetric(float64(kp.uniformRemoved), "compacted/op")
						b.ReportMetric(float64(len(kp.preds)), "predecessors/op")
						b.ReportMetric(float64(len(breaks)), "lines/op")
						b.ReportMetric(float64(q.totalCost), "demerits/op")
					}
				})
			}
		}
	}
}
