package khipu

import (
	"fmt"
	"math"
	"sort"

	"github.com/npillmayer/khipu/dimen"
)

type widthFit uint8

const (
	widthFits widthFit = iota
	widthUnderfull
	widthOverfull
)

// widthAdjustment describes physical allocation, not the breaker's score (which
// may include virtual emergency stretch). Residual is target minus actual width:
// positive leaves trailing space, negative protrudes at the paragraph's trailing
// edge. Ratio is requested adjustment / finite capacity, signed for shrink;
// absent capacity gives signed infinity. Fill uses ratio zero and records its
// order instead. Badness is rounded 100*abs(ratio)^3, capped at 10000; overfull
// and zero-capacity mismatches always have badness 10000.
type widthAdjustment struct {
	target, actual, residual dimen.DU
	ratio                    float64
	badness                  int
	fill                     dimen.DU
	fit                      widthFit
}

// lineWidthPlan is a numeric proposal, NOT frozen output. Content widths follow
// the visual knot sequence, with side skips represented separately as in geometry.
// An elastic textbox's proposed width still requires realization by a shaping
// service. Applying it to W alone would invalidate the retained glyph metrics.
type lineWidthPlan struct {
	leftSkip, rightSkip dimen.DU
	content             []dimen.DU
	adjustment          widthAdjustment
}

// planWidths adapts prepared geometry to the kind-independent pure allocator.
// It does not mutate geometry, enforce fixed-textbox eligibility, reshape text,
// or publish frozen data. geometryParagraph.freeze rejects unsupported non-glue
// elasticity before publishing the proposed widths as realizable output.
func (line geometryLine) planWidths() (*lineWidthPlan, error) {
	items := make([]KnotCore, 0, len(line.content.knots)+2)
	items = append(items, line.leftSkip)
	var contentWidth int64
	for _, k := range line.content.knots {
		// A valid geometry snapshot has a finite nonnegative content total.
		w := int64(k.core.W)
		if (w > 0 && contentWidth > math.MaxInt64-w) || (w < 0 && contentWidth < math.MinInt64-w) {
			return nil, fmt.Errorf("%w: width plan content sum overflow", ErrIllegalArguments)
		}
		contentWidth += w
		items = append(items, k.core)
	}
	if contentWidth < 0 || contentWidth >= int64(dimen.Fil) || contentWidth != int64(line.content.naturalWidth) ||
		contentWidth+int64(line.leftSkip.W)+int64(line.rightSkip.W) != int64(line.naturalWidth) ||
		int64(line.targetWidth)-int64(line.naturalWidth) != int64(line.delta) {
		return nil, fmt.Errorf("%w: inconsistent width-plan geometry", ErrIllegalArguments)
	}
	items = append(items, line.rightSkip)
	widths, adjustment, err := allocateWidths(items, line.targetWidth)
	if err != nil {
		return nil, err
	}
	return &lineWidthPlan{leftSkip: widths[0], rightSkip: widths[len(widths)-1], content: widths[1 : len(widths)-1], adjustment: adjustment}, nil
}

// allocateWidths assigns a finite target across elastic width triples. Kind,
// flags, penalties, source and glyph information play no part in the arithmetic.
// This permits testing future textbox allocation without pretending to realize
// kashida or other adjustments in stored glyphs. Inputs are never modified and
// failures return no partial assignment.
//
// Stretch uses only the highest present fill order (Filll > Fill > Fil); items
// of that order have equal weights because no separate fill weights exist in
// KnotCore. All other items stay at W. With no fill, weights are MaxW-W.
// Shrink always uses finite W-MinW capacities, even on items with fill maxima.
// Infinite shrink, Infinity and nonfinite natural widths remain unsupported.
//
// Finite shares are proportional to capacity. Integer floors are followed by
// largest-remainder apportionment, ties in input order (physical left-to-right
// for prepared geometry). Finite stretch may exceed MaxW; shrink stops at MinW.
// Without stretch the natural widths remain. Capacity shortfalls are outcomes,
// not errors. Width apportionment uses no floating-point arithmetic; only the
// diagnostic ratio and badness do. Complexity is
// O(N log N) time and O(N) memory, including the independently owned result.
func allocateWidths(items []KnotCore, target dimen.DU) ([]dimen.DU, widthAdjustment, error) {
	bad := func(reason string) ([]dimen.DU, widthAdjustment, error) {
		return nil, widthAdjustment{}, fmt.Errorf("%w: width allocation: %s", ErrIllegalArguments, reason)
	}
	if target < 0 || target >= dimen.Fil {
		return bad("invalid target")
	}
	widths := make([]dimen.DU, len(items))
	var natural int64
	var fill dimen.DU
	for i, k := range items {
		if !validGeometryWidths(k) {
			return bad(fmt.Sprintf("invalid widths at item %d", i))
		}
		w := int64(k.W)
		if (w > 0 && natural > math.MaxInt64-w) || (w < 0 && natural < math.MinInt64-w) {
			return bad("natural sum overflow")
		}
		natural += w
		widths[i] = k.W
		if k.MaxW >= dimen.Fil && k.MaxW > fill {
			fill = k.MaxW
		}
	}
	if natural < 0 {
		return nil, widthAdjustment{}, fmt.Errorf("%w: negative natural width", ErrUnsupportedPolicy)
	}
	if natural >= int64(dimen.Fil) {
		return bad("natural width out of range")
	}
	delta := int64(target) - natural
	outcome := widthAdjustment{target: target, actual: target}
	if delta == 0 {
		return widths, outcome, nil
	}
	amount, sign := delta, int64(1)
	if delta < 0 {
		amount, sign = -delta, -1
	}
	weights := make([]int64, len(items))
	var total int64
	for i, k := range items {
		switch {
		case delta < 0:
			weights[i] = int64(k.W) - int64(k.MinW)
		case fill != 0:
			if k.MaxW == fill {
				weights[i] = 1
			}
		default:
			weights[i] = int64(k.MaxW) - int64(k.W)
		}
		if total > math.MaxInt64-weights[i] {
			return bad("capacity sum overflow")
		}
		total += weights[i]
	}
	if delta > 0 && fill != 0 {
		outcome.fill = fill
	} else {
		outcome.ratio = math.Inf(int(sign))
		if total > 0 {
			outcome.ratio = float64(delta) / float64(total)
		}
		outcome.badness = int(math.Round(math.Min(10000, 100*math.Pow(math.Abs(outcome.ratio), 3))))
		if amount > total {
			if delta > 0 {
				outcome.fit = widthUnderfull
			} else {
				outcome.fit = widthOverfull
				outcome.badness = 10000
				amount = total // shrink only as far as the declared minima
			}
		}
	}
	if total == 0 {
		outcome.actual, outcome.residual = dimen.DU(natural), dimen.DU(delta)
		return widths, outcome, nil
	}
	outcome.actual = dimen.DU(natural + sign*amount)
	outcome.residual = target - outcome.actual
	type remainder struct {
		index int
		value int64
	}
	remainders := make([]remainder, 0, len(items))
	remaining := amount
	for i, weight := range weights {
		if weight == 0 {
			continue
		}
		// amount < Fil (<2^31), finite capacity <2*Fil (<2^32).
		// Their product fits int64; fill weights are only 1.
		product := amount * weight
		share, rest := product/total, product%total
		assigned := int64(items[i].W) + sign*share
		if assigned <= -int64(dimen.Fil) || assigned >= int64(dimen.Fil) {
			return bad(fmt.Sprintf("assigned width at item %d is not representable as finite DU", i))
		}
		widths[i] = dimen.DU(assigned)
		remaining -= share
		if rest != 0 {
			remainders = append(remainders, remainder{i, rest})
		}
	}
	sort.Slice(remainders, func(i, j int) bool {
		if remainders[i].value == remainders[j].value {
			return remainders[i].index < remainders[j].index
		}
		return remainders[i].value > remainders[j].value
	})
	for i := int64(0); i < remaining; i++ {
		// For valid quotas the residue is smaller than the number of positive
		// remainders. Each item receives at most one extra DU; shrink stays at or
		// above MinW, while finite stretch deliberately may exceed MaxW.
		at := remainders[i].index
		assigned := int64(widths[at]) + sign
		if assigned <= -int64(dimen.Fil) || assigned >= int64(dimen.Fil) {
			return bad(fmt.Sprintf("rounded width at item %d is not finite", at))
		}
		widths[at] = dimen.DU(assigned)
	}
	return widths, outcome, nil
}
