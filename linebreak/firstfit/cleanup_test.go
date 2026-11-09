package firstfit

import (
	"maps"
	"slices"
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/schuko/tracing"
)

// Count even disabled calls, so guards rather than tracer-side filtering are
// what the test verifies. The process-global selector forbids parallel use.
type recordingTrace struct {
	tracing.Trace
	level       tracing.TraceLevel
	debug, info int
}

func (tr *recordingTrace) GetTraceLevel() tracing.TraceLevel { return tr.level }
func (tr *recordingTrace) Debugf(string, ...interface{})     { tr.debug++ }
func (tr *recordingTrace) Infof(string, ...interface{})      { tr.info++ }

func TestLoggingGuardsPreserveResults(t *testing.T) {
	t.Cleanup(func() { tracing.SetTraceSelector(nil) })
	for _, mode := range []string{"ordinary", "declining", "productive"} {
		var want []int
		var selections map[int]khipu.DiscretionarySelection
		for _, level := range []tracing.TraceLevel{tracing.LevelError, tracing.LevelInfo, tracing.LevelDebug} {
			// Encode and populate candidates before installing the recorder:
			// provider/shaper tracing is outside this breaker's logging contract.
			tracing.SetTraceSelector(nil)
			k, params, _, _ := firstfitWorkload(t, 16, mode)
			layout := DefaultLayout(linebreak.RectangularParShape(40 * dimen.PT))
			if _, err := BreakParagraph(k, layout, params); err != nil {
				t.Fatal(err)
			}
			params.DiscretionaryProvider = nil
			tr := &recordingTrace{Trace: tracing.NoOpTrace(), level: level}
			tracing.SetTraceSelector(tracing.SelectorForAdapter(func() tracing.Trace { return tr }))
			got, err := BreakParagraph(k, layout, params)
			if err != nil {
				t.Fatal(err)
			}
			if level == tracing.LevelError {
				want, selections = got, maps.Clone(k.SelectedDiscretionaries)
			}
			if !slices.Equal(got, want) || !maps.Equal(selections, k.SelectedDiscretionaries) {
				t.Fatal("logging changed result")
			}
			// Exercise the info-only overfull path too.
			overfull := newKhipu([]node{{w: 30, minw: 30, maxw: 30, kind: khipu.KTTextBox}})
			if got, err := BreakParagraph(overfull, DefaultLayout(linebreak.RectangularParShape(20*dimen.BP)), nil); err != nil || !slices.Equal(got, []int{0}) {
				t.Fatal(got, err)
			}
			if (tr.debug > 0) != (level >= tracing.LevelDebug) || (tr.info > 0) != (level >= tracing.LevelInfo) {
				t.Fatalf("level=%v debug=%d info=%d", level, tr.debug, tr.info)
			}
		}
	}
}

func TestForcedTextboxDoesNotRequestRescue(t *testing.T) {
	k := newKhipu([]node{
		{w: 10, minw: 10, maxw: 10, kind: khipu.KTTextBox},
		{w: 30, minw: 30, maxw: 30, p: InfinityMerits, kind: khipu.KTTextBox},
		{maxw: dimen.Fill, p: InfinityMerits, kind: khipu.KTGlue},
	})
	k.Flags[2] = 0
	p := &testDiscretionaryProvider{want: []khipu.DiscretionaryCandidate{policyCandidate(1, 5*dimen.BP, 0)}}
	params := NewDefaultParameters()
	params.DiscretionaryProvider = p
	got, err := BreakParagraph(k, DefaultLayout(linebreak.RectangularParShape(20*dimen.BP)), params)
	if err != nil || !slices.Equal(got, []int{0, 1, 2}) || p.calls != 0 || len(k.SelectedDiscretionaries) != 0 {
		t.Fatal(got, err, p.calls)
	}
}

func TestKernFlagsControlCheckpointAndCarry(t *testing.T) {
	for _, flagged := range []bool{false, true} {
		k := newKhipu([]node{
			{w: 10, minw: 10, maxw: 10, kind: khipu.KTTextBox},
			{w: 15, minw: 15, maxw: 15, kind: khipu.KTKern},
			{w: 10, minw: 10, maxw: 10, kind: khipu.KTTextBox},
			{maxw: dimen.Fill, p: InfinityMerits, kind: khipu.KTGlue},
		})
		k.Flags[1], k.Flags[3] = 0, 0
		want := []int{0, 1, 3}
		if flagged {
			k.Flags[1] = khipu.KFDiscardable
			want = []int{1, 3}
		}
		got, err := BreakParagraph(k, DefaultLayout(linebreak.RectangularParShape(20*dimen.BP)), nil)
		if err != nil || !slices.Equal(got, want) {
			t.Fatalf("flagged=%v got=%v err=%v want=%v", flagged, got, err, want)
		}
	}
}

func TestKernInteriorAndLeadingCarry(t *testing.T) {
	// The prohibited kern cannot be a checkpoint. It counts when interior,
	// forcing the earlier break, but disappears at the start of the carried line.
	k := newKhipu([]node{
		{w: 10, minw: 10, maxw: 10, kind: khipu.KTTextBox},
		{w: 15, minw: 15, maxw: 15, p: InfinityDemerits, kind: khipu.KTKern},
		{w: 10, minw: 10, maxw: 10, kind: khipu.KTTextBox},
		{w: 10, minw: 10, maxw: 10, kind: khipu.KTTextBox},
		{maxw: dimen.Fill, p: InfinityMerits, kind: khipu.KTGlue},
	})
	k.Flags[4] = 0
	got, err := BreakParagraph(k, DefaultLayout(linebreak.RectangularParShape(20*dimen.BP)), nil)
	if err != nil || !slices.Equal(got, []int{0, 4}) {
		t.Fatal(got, err)
	}
}
