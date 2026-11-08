package knuthplass

import (
	"math"
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
)

func TestEmergencyClassificationAndCandidateConsistency(t *testing.T) {
	for _, tc := range []struct {
		name          string
		w             WSS
		target, extra dimen.DU
		tolerance     merits
		ratio         float64
		badness       merits
		disposition   lineDisposition
	}{
		{"exact fit", WSS{W: 100, Min: 100, Max: 100}, 100, 20, 0, 0, 0, lineAccepted},
		{"finite stretch", WSS{W: 90, Min: 80, Max: 95}, 100, 5, 100, 1, 100, lineAccepted},
		{"zero real stretch", WSS{W: 90, Min: 80, Max: 90}, 100, 20, 12, .5, 12, lineAccepted},
		{"still above tolerance", WSS{W: 90, Min: 80, Max: 90}, 100, 20, 11, .5, 12, lineScreenedOut},
		{"badness still capped", WSS{W: 90, Min: 80, Max: 90}, 100, 1, 200, 10, 10000, lineScreenedOut},
		{"empty line", WSS{}, 100, 100, 100, 1, 100, lineAccepted},
		{"shrink unchanged", WSS{W: 110, Min: 90, Max: 120}, 100, 50, 12, -.5, 12, lineAccepted},
		{"shrink limit", WSS{W: 110, Min: 100, Max: 120}, 100, 50, 100, -1, 100, lineAccepted},
		{"overfull unchanged", WSS{W: 111, Min: 101, Max: 120}, 100, 50, 10000, -1.1, 10000, lineOverfull},
		{"no shrink", WSS{W: 110, Min: 110, Max: 120}, 100, 50, 10000, math.Inf(-1), 10000, lineOverfull},
		// The new denominator exceeds DU and the symbolic fill range but is
		// still finite. Clamping/narrowing it would produce the wrong badness.
		{"large finite denominator", WSS{Max: dimen.Fil - 1}, dimen.Fil - 1, dimen.Fil - 1, 12, .5, 12, lineAccepted},
		// Both subtractions exceed signed DU. Only the stored adjustment
		// magnitude saturates; all arithmetic feeding the ratio remains wide.
		{"wide subtractions", WSS{W: -1500000000, Min: -1500000000, Max: 1500000000}, 1500000000, 1500000000, 29, 2.0 / 3.0, 29, lineAccepted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			adjustment := int64(tc.target) - int64(tc.w.W)
			if adjustment < 0 {
				adjustment = -adjustment
			}
			want := lineEvaluation{ratio: tc.ratio, badness: tc.badness, disposition: tc.disposition,
				stretch: dimen.DU(min(adjustment, int64(math.MaxInt32)))}
			if got := classifyLine(tc.w, tc.target, tc.tolerance, tc.extra); got != want {
				t.Fatalf("got %+v, want %+v", got, want)
			}
			params := NewDefaultParameters()
			params.PreTolerance, params.Tolerance = tc.tolerance, tc.tolerance
			params.FirstHyphenDemerits = 37
			kp, err := prepareLineBreaker(khipu.Params{LineShape: linebreak.RectangularParShape(tc.target)}, params)
			if err != nil {
				t.Fatal(err)
			}
			kp.hyphenating, kp.emergencyStretch = true, tc.extra
			kp.paths[kp.root][0] = bookkeeping{segment: tc.w, seenContent: true}
			before := kp.paths[kp.root][0]
			const penalty = 7
			want.bp = kp.root
			if want.accepted() {
				want.demerits = calcDemerits(want.badness, penalty, params)
			}
			kp.boundaryEvals.begin()
			for range 2 { // cached ordinary scoring must retain the same virtual geometry
				evals := kp.evaluateCandidates(kp.root, penalty)
				if len(evals) != 1 || evals[0] != want || kp.paths[kp.root][0] != before {
					t.Fatal("ordinary mismatch or modified bookkeeping", evals)
				}
			}
			kp.boundaryEvals.end()
			kp.paths[kp.root][0] = bookkeeping{}
			khp := newKhipu([]node{{w: 1, minw: 1, maxw: 1, kind: khipu.KTTextBox}})
			original := khp.KnotByIndex(0)
			post := khipu.KnotCore{Kind: khipu.KTTextBox, W: 10, MinW: 10, MaxW: 10}
			cand := khipu.DiscretionaryCandidate{Variant: 1, PostBreak: post,
				PreBreak: khipu.KnotCore{Kind: khipu.KTTextBox, W: tc.w.W, MinW: tc.w.Min, MaxW: tc.w.Max, Penalty: penalty}}
			khp.AddDiscretionaryCandidate(0, cand)
			for _, boundary := range []kinx{-1, 0} { // lookahead and seed replay
				got := kp.evaluateDiscretionaryCandidates(khp, kp.root, boundary, 0)
				if !want.accepted() {
					if len(got) != 0 {
						t.Fatal("rejected geometry admitted a discretionary")
					}
				} else {
					disc := want
					disc.demerits += params.FirstHyphenDemerits
					if len(got) != 1 || got[0].eval != disc || got[0].seed != seedBookkeepingFromKnot(post) || got[0].initial != seedBookkeepingFromKnot(post) {
						t.Fatal("discretionary mismatch or modified post-break seed", got)
					}
				}
			}
			if khp.KnotByIndex(0) != original || khp.DiscretionariesAt(0)[0] != cand || kp.seeds[origin{kp.root, 0}] != (bookkeeping{}) {
				t.Fatal("virtual capacity leaked into knots, fragments or seeds")
			}
		})
	}
}

func TestEmergencyFillCompatibility(t *testing.T) {
	for _, maximum := range []dimen.DU{dimen.Fil, dimen.Fill, dimen.Filll, dimen.Infinity} {
		for _, target := range []dimen.DU{100, dimen.Fil - 1} {
			w := WSS{Max: maximum}
			want := classifyLine(w, target, 200, 0)
			for _, extra := range []dimen.DU{1, 100, dimen.Fil - 1} {
				if got := classifyLine(w, target, 200, extra); got != want {
					t.Fatalf("Max=%d target=%d E=%d changed existing fill result: %+v vs %+v", maximum, target, extra, got, want)
				}
			}
			if target == dimen.Fil-1 && want.badness == 0 {
				t.Fatal("compatibility test must not accidentally assert exact symbolic fill semantics")
			}
		}
	}
}

func TestNonpositiveEmergencyStretchPreservesLegacyClassification(t *testing.T) {
	// Reconstruct the pre-emergency classification from its unchanged helpers,
	// including extreme summaries. E=0 must not incidentally repair old behavior.
	for _, w := range []WSS{
		{},
		{W: 100, Min: 90, Max: 110},
		{W: 110, Min: 110, Max: 110},
		{Max: dimen.Fill},
		{W: -1500000000, Min: -1500000000, Max: 1500000000},
	} {
		for _, target := range []dimen.DU{0, 90, 100, 1500000000} {
			ratio, ok := calcAdjustmentRatio(w, target)
			if !ok {
				ratio = math.Inf(1)
				if w.W > target {
					ratio = math.Inf(-1)
				}
			}
			want := lineEvaluation{ratio: ratio, stretch: absD(target - w.W), badness: 10000, disposition: lineOverfull}
			if w.Min <= target {
				want.badness, want.disposition = badnessForRatio(ratio), lineScreenedOut
				if want.badness <= 200 {
					want.disposition = lineAccepted
				}
			}
			for _, extra := range []dimen.DU{0, -1, math.MinInt32} {
				if got := classifyLine(w, target, 200, extra); got != want {
					t.Fatal("nonpositive E changed classification", w, target, extra, got, want)
				}
			}
		}
	}
}

func TestPassConfigurationControlsEmergencyStretch(t *testing.T) {
	params := NewDefaultParameters()
	params.EmergencyStretch = 100 * dimen.BP
	layout := khipu.Params{LineShape: linebreak.RectangularParShape(100 * dimen.BP)}
	khp := newKhipu([]node{
		{w: 10, minw: 10, maxw: 10, p: InfinityMerits, kind: khipu.KTTextBox},
		{maxw: dimen.Fill, p: InfinityMerits, kind: khipu.KTGlue},
	})
	if _, _, ok, err := breakParagraphPass(khp, layout, params, passConfig{id: passPreTolerance}); err != nil || ok {
		t.Fatal("first pass unexpectedly used EmergencyStretch", ok, err)
	}
	if _, _, ok, err := breakParagraphPass(khp, layout, params, passConfig{id: passTolerance, hyphenating: true}); err != nil || ok {
		t.Fatal("non-final second pass used emergency stretch or fallback", ok, err)
	}
	if _, q, ok, err := breakParagraphPass(khp, layout, params, passConfig{id: passTolerance, hyphenating: true, final: true}); err != nil || !ok || q.worstBadness != 10000 {
		t.Fatal("explicit final second pass changed its fallback behavior", q, ok, err)
	}
}
