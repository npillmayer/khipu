package khipu_test

import (
	"errors"
	"reflect"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/khipu/linebreak/firstfit"
	"github.com/npillmayer/khipu/linebreak/knuthplass"
	"github.com/npillmayer/uax/bidi"
)

func trialText(t *testing.T, s string) *styled.Paragraph {
	t.Helper()
	text := styled.TextFromString(s)
	p, e := styled.ParagraphFromText(&text, 0, text.Raw().Len(), bidi.LeftToRight, nil)
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func prepareTrial(t *testing.T, q *khipu.Khipukamayuq, s string, mode khipu.HyphenationMode) (*khipu.Khipu, *khipu.Discovery, *khipu.AtomicBinding) {
	t.Helper()
	k, d, e := q.PrepareParagraph(khipu.ParagraphInput{Paragraph: trialText(t, s)}, khipu.PrepareOptions{Hyphenation: mode})
	if e != nil {
		t.Fatal(e)
	}
	b, e := khipu.BindParagraph(k, nil)
	if e != nil {
		t.Fatal(e)
	}
	return k, d, b
}
func trialRequest(b *khipu.AtomicBinding, d *khipu.Discovery, w dimen.DU) khipu.BreakRequest {
	return exampleRequest(b, d, firstfit.DefaultLayout(linebreak.RectangularParShape(w)))
}
func frozenSnapshot(t *testing.T, p *khipu.Tikray) []any {
	t.Helper()
	f, e := khipu.Freeze(p)
	if e != nil {
		t.Fatal(e)
	}
	var out []any
	for i := 0; i < f.LineCount(); i++ {
		l, _ := f.Line(i)
		out = append(out, l)
	}
	for i := 0; i < f.KnotCount(); i++ {
		k, _ := f.Knot(i)
		text, _ := f.CopyText(i)
		out = append(out, k, text)
	}
	return out
}

type trialHyphenator struct {
	calls atomic.Int32
	fail  atomic.Bool
}

func (h *trialHyphenator) Discretionaries(s string, _ *khipu.Params) ([]khipu.HyphenationFragment, error) {
	h.calls.Add(1)
	if h.fail.Load() {
		return nil, errTrialDiscovery
	}
	if s == "\u00ad" {
		return []khipu.HyphenationFragment{{PreBreak: "-", Variant: 99}, {PreBreak: "=", Variant: 99, Penalty: khipu.MaxPenalty}}, nil
	}
	return []khipu.HyphenationFragment{{PreBreak: "ab-", PostBreak: "cd", SourceSplit: khipu.SourceSplit{ByteOffset: 2, Valid: true}}, {Variant: 1, PreBreak: "ab=", PostBreak: "cd", SourceSplit: khipu.SourceSplit{ByteOffset: 2, Valid: true}}}, nil
}

var errTrialDiscovery = errors.New("temporary discovery failure")

type countedShape struct {
	width  dimen.DU
	visits map[int32]int
	forbid bool
}

func (s *countedShape) LineLength(n int32) dimen.DU {
	if s.forbid {
		panic("unexpected geometry callback")
	}
	s.visits[n]++
	if s.visits[n] > 1 {
		panic("width queried twice")
	}
	return s.width
}

func TestImmutableMaterialAndAtomicBindings(t *testing.T) {
	q := khipu.NewKhipukamayuq(khipu.Config{})
	in := khipu.ParagraphInput{Paragraph: trialText(t, "a\uFFFC b\uFFFC c"), Atomics: []contracts.AtomicAnchor{{ID: 7, ByteOffset: 1}, {ID: 8, ByteOffset: 6}}}
	k, d, e := q.PrepareParagraph(in, khipu.PrepareOptions{})
	if e != nil {
		t.Fatal(e)
	}
	same, _, e := q.PrepareParagraph(in, khipu.PrepareOptions{})
	if e != nil || same.ID() == k.ID() {
		t.Fatal("incarnations reused", e)
	}
	ids := k.AtomicIDs()
	ids[0] = 90
	if !slices.Equal(k.AtomicIDs(), []contracts.AtomicID{7, 8}) {
		t.Fatal("atomic IDs alias")
	}
	input := []khipu.AtomicAdvance{{ID: 8, Advance: -3}, {ID: 7, Advance: 0}}
	b, e := khipu.BindParagraph(k, input)
	if e != nil {
		t.Fatal(e)
	}
	input[0].Advance = 99
	for i := 0; i < k.KnotCount(); i++ {
		base, _ := k.Knot(i)
		bound, _ := b.Knot(i)
		if base.Atomic != 0 {
			if base.MetricsKnown || base.Core.W != 0 || bound.MinW != bound.W || bound.MaxW != bound.W {
				t.Fatal(base, bound)
			}
		}
		base.Core.W = 99
		again, _ := k.Knot(i)
		if again.Core.W == 99 {
			t.Fatal("template alias")
		}
	}
	a := b.AtomicAdvances()
	a[0].Advance = 42
	if b.AtomicAdvances()[0].Advance != 0 || b.AtomicAdvances()[1].Advance != -3 {
		t.Fatal("binding alias")
	}
	for _, bad := range [][]khipu.AtomicAdvance{nil, {{ID: 7}, {ID: 7}}, {{ID: 7}, {ID: 99}}, {{ID: 7}, {ID: 8, Advance: dimen.Fil}}, {{ID: 7}, {ID: 8, Advance: -dimen.Fil}}} {
		if v, e := khipu.BindParagraph(k, bad); v != nil || !errors.Is(e, khipu.ErrIllegalArguments) {
			t.Fatal(v, e)
		}
	}
	if e := trialRequest(b, d, 100).Validate(); e != nil {
		t.Fatal(e)
	}
	for _, i := range []int{-1, k.KnotCount()} {
		if _, ok := k.Knot(i); ok {
			t.Fatal(i)
		}
		if _, ok := b.Knot(i); ok {
			t.Fatal(i)
		}
	}
}

func TestIndependentTikraysCaptureGeometryAndSurviveLaterOutcomes(t *testing.T) {
	for _, algorithm := range []string{"firstfit", "knuthplass"} {
		t.Run(algorithm, func(t *testing.T) {
			h := new(trialHyphenator)
			q := khipu.NewKhipukamayuq(khipu.Config{Hyphenator: h})
			_, d, b := prepareTrial(t, q, "xx ab\u00adcd ef\u00adgh", khipu.ExplicitHyphenation)
			run := func(r khipu.BreakRequest) (*khipu.Tikray, error) {
				if algorithm == "firstfit" {
					return firstfit.BreakParagraph(r, nil)
				}
				return knuthplass.BreakParagraph(r, nil)
			}
			shape := &countedShape{width: 30 * dimen.BP, visits: make(map[int32]int)}
			r := trialRequest(b, d, 0)
			r.Geometry.Shape = shape
			a, e := run(r)
			if e != nil {
				t.Fatal(e)
			}
			shape.forbid = true
			want := frozenSnapshot(t, a)
			lines := a.Lines()
			if len(lines) < 2 {
				t.Fatal("fixture needs several lines")
			}
			lines[0].Geometry.TargetWidth = 0
			policies := a.Policies()
			policies[0].Algorithm = 0
			if _, e = run(trialRequest(b, d, 120*dimen.BP)); e != nil {
				t.Fatal(e)
			}
			bad := trialRequest(b, d, -1)
			if p, e := run(bad); p != nil || e == nil {
				t.Fatal("bad geometry", p, e)
			}
			if got := frozenSnapshot(t, a); !reflect.DeepEqual(got, want) {
				t.Fatal("later trial changed A")
			}
			for _, i := range []int{-1, len(a.Lines())} {
				if v, ok := a.PolicyForLine(i); ok || v != (khipu.PolicyRecord{}) {
					t.Fatal(v, ok)
				}
			}
			diag := a.Diagnostics()
			if diag.HasScore != (algorithm == "knuthplass") || diag.HasPass != (algorithm == "knuthplass") {
				t.Fatal(diag)
			}
			// Publication uses captured values only; the shape is forbidden now.
			p, e := khipu.NewTikray(r, a.Lines(), a.Policies()[0], diag)
			if e != nil || !reflect.DeepEqual(frozenSnapshot(t, p), want) {
				t.Fatal(e)
			}
		})
	}
	for _, p := range []*khipu.Tikray{nil, {}} {
		if p.Diagnostics() != (khipu.BreakDiagnostics{}) {
			t.Fatal("zero diagnostics")
		}
		if _, ok := p.PolicyForLine(0); ok {
			t.Fatal("zero policy")
		}
		if f, e := khipu.Freeze(p); f != nil || !errors.Is(e, khipu.ErrIllegalArguments) {
			t.Fatal(f, e)
		}
	}
}

func TestDiscoveryNormalizationOwnershipAndConcurrentPublication(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		h := new(trialHyphenator)
		q := khipu.NewKhipukamayuq(khipu.Config{Hyphenator: h})
		s := "abcd"
		mode := khipu.AutomaticHyphenation
		if explicit {
			s = "ab\u00adcd"
			mode = khipu.ExplicitHyphenation
		}
		k, d, _ := prepareTrial(t, q, s, mode)
		h.fail.Store(true)
		for range 2 {
			if a, e := d.Alternatives(0); a != nil || !errors.Is(e, errTrialDiscovery) {
				t.Fatal(a, e)
			}
		}
		h.fail.Store(false)
		var wg sync.WaitGroup
		sets := make([]*khipu.AlternativeSet, 12)
		for i := range sets {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				var e error
				sets[i], e = d.Alternatives(0)
				if e != nil {
					t.Error(e)
				}
			}(i)
		}
		wg.Wait()
		if h.calls.Load() != 3 {
			t.Fatal("duplicate discovery", h.calls.Load())
		}
		for _, a := range sets {
			if a != sets[0] {
				t.Fatal("batch not shared")
			}
		}
		ids := sets[0].IDs()
		want := []uint16{2, 1}
		if explicit {
			want = []uint16{1, 2}
		}
		for i, id := range ids {
			if id.Incarnation != k.ID() || id.Variant != want[i] {
				t.Fatal(ids)
			}
		}
		values := sets[0].Candidates()
		values[0].PreBreak.W = 999
		ids[0].Variant = 0
		if sets[0].IDs()[0].Variant == 0 || sets[0].Candidates()[0].PreBreak.W == 999 {
			t.Fatal("candidate getter aliases cache")
		}
	}
}

func TestTikrayRejectsIllegalPathsAndForgedAlternatives(t *testing.T) {
	_, d, b := prepareTrial(t, khipu.NewKhipukamayuq(khipu.Config{}), "aa bb\ncc dd", khipu.NoHyphenation)
	r := trialRequest(b, d, 25*dimen.BP)
	p, e := firstfit.BreakParagraph(r, nil)
	if e != nil {
		t.Fatal(e)
	}
	valid := p.Lines()
	policy := p.Policies()[0]
	for _, mutate := range []func([]khipu.LineDecision) []khipu.LineDecision{
		func(v []khipu.LineDecision) []khipu.LineDecision { return nil },
		func(v []khipu.LineDecision) []khipu.LineDecision { return v[:len(v)-1] },
		func(v []khipu.LineDecision) []khipu.LineDecision { v[0].End.Kind = 99; return v },
		func(v []khipu.LineDecision) []khipu.LineDecision { v[0].Geometry.TargetWidth = dimen.Fil; return v },
		func(v []khipu.LineDecision) []khipu.LineDecision {
			v[0].End.Alternative = khipu.AlternativeID{Incarnation: b.Incarnation(), Variant: 1}
			return v
		},
		func(v []khipu.LineDecision) []khipu.LineDecision { return v[len(v)-1:] },
	} {
		if got, e := khipu.NewTikray(r, mutate(slices.Clone(valid)), policy, khipu.BreakDiagnostics{}); got != nil || !errors.Is(e, khipu.ErrIllegalArguments) {
			t.Fatal(got, e)
		}
	}
	policy.KnuthPlass.Tolerance = 1
	if got, e := khipu.NewTikray(r, valid, policy, khipu.BreakDiagnostics{}); got != nil || e == nil {
		t.Fatal("inactive policy", e)
	}
}

func TestCheckpointAtomicCompatibilityAndDetachedSeeds(t *testing.T) {
	q := khipu.NewKhipukamayuq(khipu.Config{})
	source := "\uFFFC aa\n\uFFFC bb"
	k, d, e := q.PrepareParagraph(khipu.ParagraphInput{Paragraph: trialText(t, source), Atomics: []contracts.AtomicAnchor{{ID: 1}, {ID: 2, ByteOffset: 7}}}, khipu.PrepareOptions{})
	if e != nil {
		t.Fatal(e)
	}
	bind := func(a, z dimen.DU) *khipu.AtomicBinding {
		b, e := khipu.BindParagraph(k, []khipu.AtomicAdvance{{ID: 1, Advance: a}, {ID: 2, Advance: z}})
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	b := bind(5, 5)
	r := trialRequest(b, d, 100*dimen.BP)
	p, e := knuthplass.BreakParagraph(r, nil)
	if e != nil {
		t.Fatal(e)
	}
	c, e := khipu.Checkpoint(p, 1)
	if e != nil {
		t.Fatal(e)
	}
	if _, e := c.SeedFor(bind(6, 5)); !errors.Is(e, khipu.ErrIllegalArguments) {
		t.Fatal("consumed atom changed", e)
	}
	suffix := bind(5, 9)
	seed, e := c.SeedFor(suffix)
	if e != nil {
		t.Fatal(e)
	}
	seed.Prefix[0].End.Knot = 0
	seed.Resume.NextKnot = 0
	r.Binding = suffix
	r.Prefix = c
	shape := &countedShape{width: 20 * dimen.BP, visits: make(map[int32]int)}
	r.Geometry.Shape = shape
	continued, e := firstfit.ContinueParagraph(r, nil)
	if e != nil {
		t.Fatal(e)
	}
	if shape.visits[1] != 0 {
		t.Fatal("prefix width queried")
	}
	if continued.Lines()[0] != p.Lines()[0] {
		t.Fatal("prefix changed")
	}
	if v, _ := continued.PolicyForLine(0); v.Algorithm != khipu.KnuthPlassAlgorithm {
		t.Fatal(v)
	}
	if v, _ := continued.PolicyForLine(1); v.Algorithm != khipu.FirstFitAlgorithm {
		t.Fatal(v)
	}
	edited := continued.Lines()
	edited[0].Geometry.TargetWidth++
	if v, e := khipu.NewTikray(r, edited, khipu.PolicyRecord{Algorithm: khipu.FirstFitAlgorithm}, khipu.BreakDiagnostics{}); v != nil || !errors.Is(e, khipu.ErrIllegalArguments) {
		t.Fatal("edited locked prefix accepted", e)
	}
	if v, e := knuthplass.BreakParagraph(r, nil); v != nil || !errors.Is(e, khipu.ErrUnsupportedPolicy) {
		t.Fatal(v, e)
	}
}

func TestFrozenResultReleasesAllWorkingHandles(t *testing.T) {
	done := make(chan string, 5)
	f := func() *khipu.FrozenKhipu {
		k, d, b := prepareTrial(t, khipu.NewKhipukamayuq(khipu.Config{}), "aa bb cc dd ee ff", khipu.NoHyphenation)
		p, e := firstfit.BreakParagraph(trialRequest(b, d, 25*dimen.BP), nil)
		if e != nil {
			t.Fatal(e)
		}
		c, e := khipu.Checkpoint(p, 1)
		if e != nil {
			t.Fatal(e)
		}
		runtime.SetFinalizer(k, func(*khipu.Khipu) { done <- "material" })
		runtime.SetFinalizer(d, func(*khipu.Discovery) { done <- "discovery" })
		runtime.SetFinalizer(b, func(*khipu.AtomicBinding) { done <- "binding" })
		runtime.SetFinalizer(p, func(*khipu.Tikray) { done <- "plan" })
		runtime.SetFinalizer(c, func(*khipu.BreakCheckpoint) { done <- "checkpoint" })
		f, e := khipu.Freeze(p)
		if e != nil {
			t.Fatal(e)
		}
		runtime.KeepAlive(c)
		return f
	}()
	received := map[string]bool{}
	deadline := time.Now().Add(5 * time.Second)
	for len(received) < 5 && time.Now().Before(deadline) {
		runtime.GC()
		select {
		case v := <-done:
			received[v] = true
		case <-time.After(10 * time.Millisecond):
		}
	}
	if len(received) != 5 {
		t.Fatal("frozen result retains working handles", received)
	}
	if f.LineCount() < 2 {
		t.Fatal("frozen result lost material")
	}
	if _, ok := f.CopyText(0); !ok {
		t.Fatal("frozen glyphs unavailable")
	}
	runtime.KeepAlive(f)
}

func TestFailedTrialAndConcurrentFreezesPreserveEarlierPlan(t *testing.T) {
	h := new(trialHyphenator)
	q := khipu.NewKhipukamayuq(khipu.Config{Hyphenator: h})
	_, d, b := prepareTrial(t, q, "xx ab\u00adcd ef\u00adgh", khipu.ExplicitHyphenation)
	a, e := firstfit.BreakParagraph(trialRequest(b, d, 200*dimen.BP), nil)
	if e != nil {
		t.Fatal(e)
	}
	want := frozenSnapshot(t, a)
	h.fail.Store(true)
	if p, e := firstfit.BreakParagraph(trialRequest(b, d, 30*dimen.BP), nil); p != nil || !errors.Is(e, errTrialDiscovery) {
		t.Fatal(p, e)
	}
	if got := frozenSnapshot(t, a); !reflect.DeepEqual(got, want) {
		t.Fatal("failed B changed A")
	}
	h.fail.Store(false)
	selected, e := firstfit.BreakParagraph(trialRequest(b, d, 30*dimen.BP), nil)
	if e != nil {
		t.Fatal(e)
	}
	selectedWant := frozenSnapshot(t, selected)
	calls := h.calls.Load()
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got := frozenSnapshot(t, selected); !reflect.DeepEqual(got, selectedWant) {
				t.Error("concurrent freeze changed replacements")
			}
			p, e := firstfit.BreakParagraph(trialRequest(b, d, 30*dimen.BP), nil)
			if e != nil {
				t.Error(e)
				return
			}
			if got := frozenSnapshot(t, p); !reflect.DeepEqual(got, selectedWant) {
				t.Error("concurrent search changed selection")
			}
		}()
	}
	wg.Wait()
	if h.calls.Load() != calls {
		t.Fatal("cached trial rediscovered alternatives")
	}
}
