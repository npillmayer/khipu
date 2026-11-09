package firstfit

import (
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
)

func TestDiscretionaryFitUsesSuppliedGlueShrink(t *testing.T) {
	for _, tc := range []struct {
		name    string
		minimum dimen.DU
		want    bool
	}{
		{"exact minimum", 4 * dimen.BP, true},
		{"one DU too wide", 4*dimen.BP + 1, false},
		{"fixed space", 5 * dimen.BP, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k := newKhipu([]node{{w: 20, minw: 20, maxw: 20, kind: khipu.KTTextBox}})
			k.AddDiscretionaryCandidate(0, policyCandidate(1, 10*dimen.BP, 50))
			base := segmentSeedFromKnot(knot(10, 10, 10, 0, khipu.KTTextBox))
			layout := DefaultLayout(linebreak.RectangularParShape(24 * dimen.BP))
			checkpoint := base.width(layout)
			space := knot(5, 5, 5, 0, khipu.KTGlue)
			space.MinW = tc.minimum
			base.append(space)
			// PreBreak makes the trailing glue interior: natural width is 25bp,
			// so only the supplied 1bp of shrink can admit it at a 24bp target.
			_, _, ok, err := tryLooseHyphenation(k, 0, base, checkpoint, 24*dimen.BP, layout, NewDefaultParameters())
			if err != nil || ok != tc.want {
				t.Fatalf("eligible=%v err=%v; want %v", ok, err, tc.want)
			}
		})
	}
}
