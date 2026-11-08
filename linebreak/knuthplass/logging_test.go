package knuthplass

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/schuko/tracing"
)

// Deliberately record even disabled calls: a tracer that silently filters them
// would not catch argument construction happening before Debugf/Infof.
type recordingKPTrace struct {
	tracing.Trace
	level       tracing.TraceLevel
	debug, info strings.Builder
	errors      strings.Builder
}

func (tr *recordingKPTrace) GetTraceLevel() tracing.TraceLevel { return tr.level }
func (tr *recordingKPTrace) Debugf(format string, args ...interface{}) {
	fmt.Fprintf(&tr.debug, format, args...)
}
func (tr *recordingKPTrace) Infof(format string, args ...interface{}) {
	fmt.Fprintf(&tr.info, format, args...)
}

func (tr *recordingKPTrace) Errorf(format string, args ...interface{}) {
	fmt.Fprintf(&tr.errors, format, args...)
}

// Like the package's QuickConfig tests, this test installs a process-global
// selector and must not run in parallel. Restore the default no-op on exit.
func TestTraceGuardsPreservePathResults(t *testing.T) {
	t.Cleanup(func() { tracing.SetTraceSelector(nil) })
	for _, hyphenating := range []bool{false, true} {
		var wantBreaks []BreakRef
		var wantQuality pathQuality
		var wantPreds map[pathState]predecessor
		for _, level := range []tracing.TraceLevel{tracing.LevelError, tracing.LevelInfo, tracing.LevelDebug} {
			trace := &recordingKPTrace{Trace: tracing.NoOpTrace(), level: level}
			tracing.SetTraceSelector(tracing.SelectorForAdapter(func() tracing.Trace { return trace }))
			khp := newKhipu([]node{
				{w: 20, minw: 20, maxw: 20, p: -100, kind: khipu.KTTextBox},
				{w: 1, minw: 1, maxw: 1, kind: khipu.KTGlue},
				{w: 20, minw: 20, maxw: 20, p: InfinityDemerits, kind: khipu.KTTextBox},
				{maxw: dimen.Fill, p: InfinityMerits, kind: khipu.KTGlue},
			})
			khp.AddDiscretionaryCandidate(2, khipu.DiscretionaryCandidate{
				Variant:   1,
				PreBreak:  khipu.KnotCore{W: 5 * dimen.BP, MinW: 5 * dimen.BP, MaxW: 5 * dimen.BP, Penalty: 50, Kind: khipu.KTTextBox},
				PostBreak: khipu.KnotCore{W: 15 * dimen.BP, MinW: 15 * dimen.BP, MaxW: 15 * dimen.BP, Kind: khipu.KTTextBox},
			})
			params := NewDefaultParameters()
			params.LinePenalty = 0
			layout := khipu.Params{LineShape: linebreak.RectangularParShape(26 * dimen.BP), RightSkip: glue(0, 0, 5*dimen.BP)}
			kp, err := prepareLineBreaker(layout, params)
			if err != nil {
				t.Fatal(err)
			}
			kp.hyphenating = hyphenating
			kp.final = true // exercise diagnostic fallback independently of discovery
			if err := kp.constructBreakpointGraph(khp); err != nil {
				t.Fatal(err)
			}
			breaks, quality, ok := kp.collectOptimalBreakRefs(kp.end)
			if !ok {
				t.Fatal("no complete path")
			}
			if level == tracing.LevelError {
				wantBreaks, wantQuality, wantPreds = breaks, quality, maps.Clone(kp.preds)
			} else if !slices.Equal(breaks, wantBreaks) || quality != wantQuality || !maps.Equal(kp.preds, wantPreds) {
				t.Fatalf("logging changed path results: hyphenating=%v level=%v", hyphenating, level)
			}
			if hyphenating && !slices.Equal(breaks, []BreakRef{{At: 2, Variant: 1}, PlainBreak(3)}) {
				t.Fatalf("fixture did not select discretionary: %v", breaks)
			}
			if level < tracing.LevelDebug && trace.debug.Len() != 0 {
				t.Fatal("disabled debug call reached tracer")
			}
			if level < tracing.LevelInfo && trace.info.Len() != 0 {
				t.Fatal("disabled info call reached tracer")
			}
			if level == tracing.LevelDebug && !strings.Contains(trace.debug.String(), " paths:\nBP[") {
				t.Fatal("enabled debug path dump missing")
			}
			if level >= tracing.LevelInfo && (!strings.Contains(trace.info.String(), "predecessors:\n") || !strings.Contains(trace.info.String(), "paths:\n")) {
				t.Fatal("enabled info state dumps missing")
			}
		}
	}
}
