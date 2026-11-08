package knuthplass

import (
	"errors"
	"maps"
	"slices"
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
)

// A forced first line isolates pass screening from the retained terminal fill.
func emergencyForcedLine(width, maximum dimen.DU) *khipu.Khipu {
	return newKhipu([]node{
		{w: width, minw: width, maxw: maximum, p: InfinityMerits, kind: khipu.KTTextBox},
		{maxw: dimen.Fill, p: InfinityMerits, kind: khipu.KTGlue},
	})
}

func TestEmergencyPassPolicy(t *testing.T) {
	if NewKPDefaultParameters().EmergencyStretch != 0 || DefaultParameters.EmergencyStretch != 50*dimen.BP {
		t.Fatal("constructor must opt out; tolerant profile must retain its explicit recovery policy")
	}
	for _, extra := range []dimen.DU{-1, 0, 1, dimen.Fil - 1} {
		params := NewKPDefaultParameters()
		params.EmergencyStretch = extra
		passes, err := paragraphPasses(params)
		want := []passConfig{{id: passPreTolerance}, {id: passTolerance, hyphenating: true, final: extra <= 0}}
		if extra > 0 {
			want = append(want, passConfig{id: passEmergency, hyphenating: true, final: true, emergencyStretch: extra})
		}
		if err != nil || !slices.Equal(passes, want) || params.EmergencyStretch != extra {
			t.Fatalf("E=%d: passes=%+v err=%v", extra, passes, err)
		}
	}
}

func TestEmergencyStretchValidatedBeforeFirstPass(t *testing.T) {
	for _, extra := range []dimen.DU{dimen.Fil, dimen.Fill, dimen.Filll, dimen.Infinity} {
		params := NewKPDefaultParameters()
		params.EmergencyStretch = extra
		provider := &testDiscretionaryProvider{}
		params.DiscretionaryProvider = provider
		khp := emergencyForcedLine(100, 100) // would otherwise succeed in pass 1
		khp.SelectedDiscretionaries = map[int]khipu.DiscretionarySelection{0: {Source: 0, Variant: 9}}
		before := maps.Clone(khp.SelectedDiscretionaries)
		shapeCalls := 0
		layout := khipu.Params{LineShape: retirementShape(func(int32) dimen.DU {
			shapeCalls++
			return 100 * dimen.BP
		})}
		breaks, err := BreakParagraph(khp, layout, params)
		if !errors.Is(err, khipu.ErrIllegalArguments) || len(breaks) != 0 || provider.calls != 0 || shapeCalls != 0 || !maps.Equal(before, khp.SelectedDiscretionaries) {
			t.Fatalf("E=%d: validation must precede search and publication: %v %v", extra, breaks, err)
		}
	}
}

func TestEmergencyPassRecoveryAndLimits(t *testing.T) {
	layout := khipu.Params{LineShape: linebreak.RectangularParShape(100 * dimen.BP)}
	for _, tc := range []struct {
		name        string
		width       dimen.DU
		extra       dimen.DU
		wantPass    passID
		wantBadness merits
		fallback    bool
	}{
		{"disabled", 90, 0, passTolerance, 10000, true},
		{"negative disabled", 90, -1, passTolerance, 10000, true},
		{"loose recovery", 90, 10, passEmergency, 100, false},
		{"insufficient emergency capacity", 90, 1, passEmergency, 10000, true},
		{"overfull still needs fallback", 150, 100, passEmergency, 10000, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params := NewKPDefaultParameters()
			params.EmergencyStretch = tc.extra * dimen.BP
			khp := emergencyForcedLine(tc.width, tc.width)
			widths, minima, maxima := slices.Clone(khp.W), slices.Clone(khp.MinW), slices.Clone(khp.MaxW)
			passes, err := paragraphPasses(params)
			if err != nil {
				t.Fatal(err)
			}
			// Check non-final failure directly: pass 2 must not hide failure
			// with fallback and thereby prevent the emergency invocation.
			for _, pass := range passes[:len(passes)-1] {
				result, err := breakParagraphPassRefs(khp, layout, params, pass)
				if err != nil || result.complete || len(result.breaks) != 0 || result.pass != pass {
					t.Fatalf("non-final pass unexpectedly completed: %+v %v", result, err)
				}
			}
			result, err := selectParagraphBreaks(khp, layout, params)
			if err != nil || !result.complete || result.pass.id != tc.wantPass || result.quality.worstBadness != tc.wantBadness || !slices.Equal(projectBreakRefs(result.breaks), []int{0, 1}) {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			wantCost := calcDemerits(tc.wantBadness, 0, params) + calcDemerits(0, 0, params)
			if tc.fallback {
				wantCost = AwfulDemerits + calcDemerits(0, 0, params)
			}
			if result.quality.totalCost != wantCost || result.pass.emergencyStretch != max(0, params.EmergencyStretch) {
				t.Fatalf("wrong scoring context or fallback cost: %+v want cost=%d", result, wantCost)
			}
			breaks, err := BreakParagraph(khp, layout, params)
			if err != nil || !slices.Equal(breaks, []int{0, 1}) || !slices.Equal(khp.W, widths) || !slices.Equal(khp.MinW, minima) || !slices.Equal(khp.MaxW, maxima) {
				t.Fatalf("public result changed real widths: breaks=%v err=%v", breaks, err)
			}
		})
	}
}

func TestEarlierSuccessfulPassWins(t *testing.T) {
	layout := khipu.Params{LineShape: linebreak.RectangularParShape(100 * dimen.BP)}
	for _, tc := range []struct {
		width, maximum dimen.DU
		pass           passID
		badness        merits
	}{
		{100, 100, passPreTolerance, 0},
		{90, 99, passTolerance, 137},
	} {
		params := NewKPDefaultParameters()
		params.EmergencyStretch = 100 * dimen.BP
		// Even an accepted total cost above AwfulDemerits is not evidence of
		// pass failure. Nor may pass 3 compete with an acceptable pass-2 path.
		params.LinePenalty = 50000
		result, err := selectParagraphBreaks(emergencyForcedLine(tc.width, tc.maximum), layout, params)
		if err != nil || !result.complete || result.pass.id != tc.pass || result.pass.emergencyStretch != 0 || result.quality.worstBadness != tc.badness || result.quality.totalCost <= AwfulDemerits {
			t.Fatalf("wrong first successful pass: %+v %v", result, err)
		}
	}
	result, err := selectParagraphBreaks(emergencyForcedLine(90, 90), layout, nil)
	if err != nil || result.pass.id != passTolerance || !result.pass.final || result.pass.emergencyStretch != 0 {
		t.Fatalf("nil parameters changed legacy two-pass policy: %+v %v", result, err)
	}
}

type emergencyProviderFunc func(*khipu.Khipu, int) ([]khipu.DiscretionaryCandidate, error)

func (f emergencyProviderFunc) DiscretionaryCandidates(khp *khipu.Khipu, at int) ([]khipu.DiscretionaryCandidate, error) {
	return f(khp, at)
}

// An early forced loose line ends both non-final searches. Emergency stretch
// makes it acceptable, exposing a later overlong textbox for fresh discovery.
func emergencyLaterDiscovery() *khipu.Khipu {
	return newKhipu([]node{
		{w: 90, minw: 90, maxw: 90, p: InfinityMerits, kind: khipu.KTTextBox},
		{w: 150, minw: 150, maxw: 150, p: InfinityDemerits, kind: khipu.KTTextBox},
		{maxw: dimen.Fill, p: InfinityMerits, kind: khipu.KTGlue},
	})
}

func TestProviderErrorsStopPassSequence(t *testing.T) {
	for _, failIn := range []passID{passTolerance, passEmergency} {
		khp := emergencyLaterDiscovery()
		if failIn == passTolerance {
			khp.W[0], khp.MinW[0], khp.MaxW[0] = 100*dimen.BP, 100*dimen.BP, 100*dimen.BP
		}
		khp.SelectedDiscretionaries = map[int]khipu.DiscretionarySelection{0: {Source: 0, Variant: 9}}
		before := maps.Clone(khp.SelectedDiscretionaries)
		want := errors.New("provider failed")
		calls := 0
		params := NewKPDefaultParameters()
		params.EmergencyStretch = 10 * dimen.BP
		params.DiscretionaryProvider = emergencyProviderFunc(func(_ *khipu.Khipu, at int) ([]khipu.DiscretionaryCandidate, error) {
			calls++
			if at != 1 {
				t.Fatalf("unexpected provider knot %d", at)
			}
			return nil, want
		})
		layout := khipu.Params{LineShape: linebreak.RectangularParShape(100 * dimen.BP)}
		result, err := selectParagraphBreaks(khp, layout, params)
		if !errors.Is(err, want) || result.pass.id != failIn || result.complete || calls != 1 {
			t.Fatalf("error retried or became fallback: %+v calls=%d err=%v", result, calls, err)
		}
		calls = 0
		breaks, err := BreakParagraph(khp, layout, params)
		if !errors.Is(err, want) || len(breaks) != 0 || calls != 1 || !maps.Equal(before, khp.SelectedDiscretionaries) {
			t.Fatal("public failure published partial selections or retried the error", breaks, err)
		}
	}
}

func TestEmergencyPublishesOnlyWinningSelections(t *testing.T) {
	khp := emergencyLaterDiscovery()
	khp.SelectedDiscretionaries = map[int]khipu.DiscretionarySelection{0: {Source: 0, Variant: 9}}
	before := maps.Clone(khp.SelectedDiscretionaries)
	provider := &testDiscretionaryProvider{want: []khipu.DiscretionaryCandidate{{
		Variant:   1,
		PreBreak:  khipu.KnotCore{Kind: khipu.KTTextBox, W: 90 * dimen.BP, MinW: 90 * dimen.BP, MaxW: 90 * dimen.BP},
		PostBreak: khipu.KnotCore{Kind: khipu.KTTextBox, W: 60 * dimen.BP, MinW: 60 * dimen.BP, MaxW: 60 * dimen.BP},
	}}}
	params := NewKPDefaultParameters()
	params.EmergencyStretch, params.DiscretionaryProvider = 10*dimen.BP, provider
	layout := khipu.Params{LineShape: linebreak.RectangularParShape(100 * dimen.BP)}
	result, err := selectParagraphBreaks(khp, layout, params)
	want := []BreakRef{PlainBreak(0), {At: 1, Variant: 1}, PlainBreak(2)}
	if err != nil || result.pass.id != passEmergency || !slices.Equal(result.breaks, want) || result.quality.worstBadness != 100 || !maps.Equal(before, khp.SelectedDiscretionaries) {
		t.Fatalf("search changed selections or failed to use emergency discretionary: %+v %v", result, err)
	}
	if provider.calls == 0 || provider.requestedAt[0] != 1 {
		t.Fatal("emergency pass did not discover newly reached textbox")
	}
	breaks, err := BreakParagraph(khp, layout, params)
	selected, ok := khp.SelectedDiscretionaryAt(1)
	if err != nil || !slices.Equal(breaks, []int{0, 1, 2}) || !ok || selected.Variant != 1 || len(khp.SelectedDiscretionaries) != 1 {
		t.Fatalf("winning selections not published: %v %+v %v", breaks, khp.SelectedDiscretionaries, err)
	}
}
