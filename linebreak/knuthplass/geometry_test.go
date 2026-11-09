package knuthplass

import (
	"math"
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
)

func TestLineClassificationAndCandidateConsistency(t *testing.T) {
	for _, tc := range []struct {
		name        string
		width       WSS
		tolerance   merits
		ratio       float64
		badness     merits
		disposition lineDisposition
	}{
		{"rigid exact fit", WSS{W: 100, Min: 100, Max: 100}, 0, 0, 0, lineAccepted},
		{"elastic exact fit", WSS{W: 100, Min: 90, Max: 120}, 0, 0, 0, lineAccepted},
		{"finite shrink", WSS{W: 110, Min: 90, Max: 120}, 200, -0.5, 12, lineAccepted},
		{"shrink limit accepted", WSS{W: 110, Min: 100, Max: 120}, 100, -1, 100, lineAccepted},
		{"shrink limit screened", WSS{W: 110, Min: 100, Max: 120}, 99, -1, 100, lineScreenedOut},
		{"rounding cannot hide excess shrink", WSS{W: 1101, Min: 101, Max: 1200}, 100, -1.001, 10000, lineOverfull},
		{"excess shrink below old tolerance", WSS{W: 111, Min: 101, Max: 120}, 200, -1.1, 10000, lineOverfull},
		{"excess shrink at maximum tolerance", WSS{W: 111, Min: 101, Max: 120}, 10000, -1.1, 10000, lineOverfull},
		{"no shrink", WSS{W: 110, Min: 110, Max: 120}, 10000, math.Inf(-1), 10000, lineOverfull},
		{"stretch at tolerance", WSS{W: 90, Min: 80, Max: 110}, 12, 0.5, 12, lineAccepted},
		{"stretch above tolerance", WSS{W: 90, Min: 80, Max: 110}, 11, 0.5, 12, lineScreenedOut},
		{"stretch beyond maximum is not overfull", WSS{W: 90, Min: 80, Max: 95}, 800, 2, 800, lineAccepted},
		{"cubic badness cap", WSS{W: 90, Min: 80, Max: 91}, 10000, 10, 10000, lineAccepted},
		{"no stretch screened", WSS{W: 90, Min: 80, Max: 90}, 200, math.Inf(1), 10000, lineScreenedOut},
		{"no stretch admitted", WSS{W: 90, Min: 80, Max: 90}, 10000, math.Inf(1), 10000, lineAccepted},
		{"empty line", WSS{}, 200, math.Inf(1), 10000, lineScreenedOut},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := lineEvaluation{
				ratio: tc.ratio, badness: tc.badness, disposition: tc.disposition,
				stretch: absD(100 - tc.width.W),
			}
			if got := classifyLine(tc.width, 100, tc.tolerance, 0); got != want {
				t.Fatalf("classification=%+v, want %+v", got, want)
			}
			params := NewDefaultParameters()
			params.PreTolerance, params.Tolerance = tc.tolerance, tc.tolerance
			params.FirstHyphenDemerits = 37
			layout := khipu.Params{LineShape: linebreak.RectangularParShape(100)}
			kp, err := prepareLineBreaker(layout, params)
			if err != nil {
				t.Fatal(err)
			}
			kp.paths[kp.root][0] = bookkeeping{segment: tc.width, seenContent: true}
			const penalty = 7
			evals := kp.evaluateCandidates(kp.root, penalty)
			ordinary := want
			ordinary.bp = kp.root
			if ordinary.accepted() {
				q := params.LinePenalty + tc.badness
				ordinary.demerits = q*q + penalty*penalty
			}
			if len(evals) != 1 || evals[0] != ordinary {
				t.Fatalf("ordinary=%+v, want %+v", evals, ordinary)
			}

			// Give the discretionary evaluator the identical width via PreBreak.
			// Test both lookahead and seed replay, without installing a provider.
			kp.paths[kp.root][0] = bookkeeping{}
			kp.hyphenating = true
			khp := newKhipu([]node{{w: 1, minw: 1, maxw: 1, kind: khipu.KTTextBox}})
			post := khipu.KnotCore{Kind: khipu.KTTextBox, W: 10, MinW: 10, MaxW: 10}
			khp.AddDiscretionaryCandidate(0, khipu.DiscretionaryCandidate{
				Variant: 1,
				PreBreak: khipu.KnotCore{
					Kind: khipu.KTTextBox, W: tc.width.W, MinW: tc.width.Min, MaxW: tc.width.Max, Penalty: penalty,
				},
				PostBreak: post,
			})
			for _, breakpoint := range []kinx{-1, 0} {
				got := kp.evaluateDiscretionaryCandidates(khp, kp.root, breakpoint, 0)
				if !ordinary.accepted() {
					if len(got) != 0 {
						t.Fatalf("breakpoint %d: rejected geometry produced discretionaries: %+v", breakpoint, got)
					}
					continue
				}
				disc := ordinary
				disc.demerits += params.FirstHyphenDemerits
				if len(got) != 1 || got[0].eval != disc || got[0].target != (BreakRef{At: 0, Variant: 1}) ||
					got[0].seed != seedBookkeepingFromKnot(post) {
					t.Fatalf("breakpoint %d: discretionary=%+v, want eval %+v and post-break seed", breakpoint, got, disc)
				}
			}
		})
	}
}

func TestSharedClassificationUsesCurrentPassTolerance(t *testing.T) {
	params := NewDefaultParameters()
	params.PreTolerance, params.Tolerance = 99, 100
	kp, err := prepareLineBreaker(khipu.Params{LineShape: linebreak.RectangularParShape(100)}, params)
	if err != nil {
		t.Fatal(err)
	}
	w := WSS{W: 110, Min: 100, Max: 110}
	khp := newKhipu([]node{{w: 1, minw: 1, maxw: 1, kind: khipu.KTTextBox}})
	khp.AddDiscretionaryCandidate(0, khipu.DiscretionaryCandidate{
		Variant:  1,
		PreBreak: khipu.KnotCore{Kind: khipu.KTTextBox, W: w.W, MinW: w.Min, MaxW: w.Max},
	})
	for _, secondPass := range []bool{false, true} {
		kp.hyphenating = secondPass
		kp.paths[kp.root][0] = bookkeeping{segment: w, seenContent: true}
		evals := kp.evaluateCandidates(kp.root, 0)
		if len(evals) != 1 || evals[0].accepted() != secondPass {
			t.Fatalf("secondPass=%v ordinary=%+v", secondPass, evals)
		}
		kp.paths[kp.root][0] = bookkeeping{}
		// Direct scoring tests the common threshold; the path search controls whether
		// discretionary scoring is invoked at all in a non-hyphenating pass.
		cands := kp.evaluateDiscretionaryCandidates(khp, kp.root, 0, 0)
		if (len(cands) == 1) != secondPass {
			t.Fatalf("secondPass=%v discretionaries=%+v", secondPass, cands)
		}
	}
}

func TestSharedClassificationDoesNotRetireWorkingStates(t *testing.T) {
	kp, err := prepareLineBreaker(DefaultLayout(linebreak.RectangularParShape(100*dimen.PT)), nil)
	if err != nil {
		t.Fatal(err)
	}
	kp.paths[kp.root][0] = bookkeeping{
		segment: WSS{W: 200 * dimen.PT, Min: 150 * dimen.PT, Max: 250 * dimen.PT}, seenContent: true,
	}
	before := kp.paths[kp.root][0]
	evals := kp.evaluateCandidates(kp.root, 0)
	if len(evals) != 1 || evals[0].disposition != lineOverfull {
		t.Fatalf("expected overfull local classification: %+v", evals)
	}
	if kp.paths[kp.root][0] != before || len(kp.seeds) != 1 || len(kp.horizon) != 1 {
		t.Fatal("local classification changed working-state lifetime")
	}
}
