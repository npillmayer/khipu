package khipu_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak/firstfit"
)

func TestTikraySideSkipsAndCheckpointPrefixes(t *testing.T) {
	_, d, b := prepareTrial(t, khipu.NewKhipukamayuq(khipu.Config{}), "aa\nbb\ncc", khipu.NoHyphenation)
	r := trialRequest(b, d, 100*dimen.BP)
	r.Geometry.LeftSkip = khipu.KnotCore{W: 10, MinW: 5, MaxW: 15}
	r.Geometry.RightSkip = khipu.KnotCore{W: 20, MinW: 15, MaxW: 25}
	original, err := firstfit.BreakParagraph(r, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(original.Lines()) != 3 {
		t.Fatal("fixture requires three forced lines")
	}
	wantOriginal := frozenSnapshot(t, original)

	// Include fresh publication, an empty checkpoint, a mixed prefix/suffix,
	// and a fully accepted paragraph with no suffix to validate.
	for _, accepted := range []int{-1, 0, 1, 3} {
		t.Run(fmt.Sprintf("accepted=%d", accepted), func(t *testing.T) {
			request := r
			prefix := 0
			if accepted >= 0 {
				request.Prefix, err = khipu.Checkpoint(original, accepted)
				if err != nil {
					t.Fatal(err)
				}
				prefix = accepted
				// New suffix geometry must not replace the certified prefix skips.
				request.Geometry.LeftSkip = khipu.KnotCore{W: 30, MinW: 25, MaxW: 35}
				request.Geometry.RightSkip = khipu.KnotCore{W: 40, MinW: 35, MaxW: 45}
			}
			shape := &countedShape{width: 100 * dimen.BP, visits: make(map[int32]int)}
			request.Geometry.Shape = shape
			var plan *khipu.Tikray
			if accepted < 0 {
				plan, err = firstfit.BreakParagraph(request, nil)
			} else {
				plan, err = firstfit.ContinueParagraph(request, nil)
			}
			if err != nil {
				t.Fatal(err)
			}
			for i := 0; i < prefix; i++ {
				if plan.Lines()[i] != original.Lines()[i] || shape.visits[int32(i+1)] != 0 {
					t.Fatal("locked prefix changed or its width was queried")
				}
			}
			shape.forbid = true // Publication and freezing must use captured geometry.
			policy := khipu.PolicyRecord{Algorithm: khipu.FirstFitAlgorithm}
			// Non-width KnotCore fields have no meaning for side skips.
			request.Geometry.LeftSkip.Penalty = khipu.MaxPenalty
			request.Geometry.RightSkip.Kind = khipu.KTTextBox
			republished, err := khipu.NewTikray(request, plan.Lines(), policy, plan.Diagnostics())
			if err != nil {
				t.Fatal(err)
			}
			want := frozenSnapshot(t, plan)
			if !reflect.DeepEqual(frozenSnapshot(t, republished), want) {
				t.Fatal("publication changed frozen geometry")
			}
			// Every width component on either side must match, including later
			// suffix lines. Each mutation stays numerically valid on its own.
			for i := prefix; i < len(plan.Lines()); i++ {
				for _, side := range []string{"left", "right"} {
					for _, component := range []string{"natural", "minimum", "maximum"} {
						t.Run(fmt.Sprintf("line=%d/%s/%s", i, side, component), func(t *testing.T) {
							lines := plan.Lines()
							skip := &lines[i].Geometry.LeftSkip
							if side == "right" {
								skip = &lines[i].Geometry.RightSkip
							}
							switch component {
							case "natural":
								skip.Natural++
							case "minimum":
								skip.Minimum++
							case "maximum":
								skip.Maximum++
							}
							got, err := khipu.NewTikray(request, lines, policy, plan.Diagnostics())
							if got != nil || !errors.Is(err, khipu.ErrIllegalArguments) {
								t.Fatalf("accepted mismatched suffix skips: plan=%v err=%v", got, err)
							}
						})
					}
				}
			}
			if prefix > 0 {
				lines := plan.Lines()
				lines[0].Geometry.LeftSkip = khipu.WidthTriple{Natural: 30, Minimum: 25, Maximum: 35}
				if got, err := khipu.NewTikray(request, lines, policy, plan.Diagnostics()); got != nil || !errors.Is(err, khipu.ErrIllegalArguments) {
					t.Fatal("accepted rewritten prefix skips", err)
				}
			}
			if !reflect.DeepEqual(frozenSnapshot(t, plan), want) || !reflect.DeepEqual(frozenSnapshot(t, original), wantOriginal) {
				t.Fatal("publication attempts changed prior plans")
			}
		})
	}
}
