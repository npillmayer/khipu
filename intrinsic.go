package khipu

import (
	"context"
	"fmt"
	"math"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
)

// IntrinsicWidths are natural inline bounds, not glue stretch/shrink capacities.
// Both are nonnegative and below dimen.Fil, with MinContent <= MaxContent.
type IntrinsicWidths struct {
	MinContent dimen.DU
	MaxContent dimen.DU
}

// AtomicIntrinsic supplies signed inline contributions, including caller-resolved
// horizontal margins. These are not border dimensions or layout elasticity.
// Endpoints must have magnitude below dimen.Fil and MinAdvance <= MaxAdvance.
type AtomicIntrinsic struct {
	ID                     contracts.AtomicID
	MinAdvance, MaxAdvance dimen.DU
}

// MeasureRequest supplies exactly one range per attached occurrence, in any ID
// order. The operation consumes the slice without retaining caller-owned storage.
type MeasureRequest struct {
	Atomics []AtomicIntrinsic
}

// MeasureIntrinsic evaluates prepared natural advances without binding,
// breaking, visual ordering or freezing. Min-content takes all supported optional
// breaks; max-content suppresses them. Both respect mandatory boundaries and trim
// discardables at span edges. Style/knot boundaries alone do not permit breaks.
// Generated paragraph fill, side skips, elasticity and breaker scoring are absent.
//
// NoHyphenation and bounded ExplicitHyphenation are supported. Explicit measurement
// lazily shapes and memoizes replacement pairs; multiple eligible positions in one
// textbox or multiple permitting variants return ErrUnsupportedPolicy. Automatic
// hyphenation remains unsupported, without a fallback to ordinary widths. Signed
// contributions are accumulated in checked int64, then normalized at the result
// boundary. Negative completed layout lines may still be unsupported by Freeze.
//
// Serialize operations on the creating owner and keep source/styles/services
// stable, as for PrepareParagraph/BindParagraph. This query neither changes the
// base material nor reads binding-local selections or discretionary caches. Context
// is checked during evaluation; every failure returns zero widths and leaves the
// preparation reusable. Ordinary evaluation is linear in knots and occurrences;
// explicit cache misses additionally prepare and shape complete replacement pairs.
func (kq *Khipukamayuq) MeasureIntrinsic(ctx context.Context, prepared *PreparedParagraph, request MeasureRequest) (IntrinsicWidths, error) {
	if ctx == nil {
		return IntrinsicWidths{}, ErrIllegalArguments
	}
	if err := ctx.Err(); err != nil {
		return IntrinsicWidths{}, err
	}
	if kq == nil {
		return IntrinsicWidths{}, ErrVoidKhipukamayuq
	}
	if prepared == nil || prepared.owner != kq || prepared.preparedMaterial == nil || prepared.paragraph == nil ||
		len(prepared.knots) == 0 || len(prepared.knots) != len(prepared.shaping.sources) || len(request.Atomics) != prepared.atomicCount {
		return IntrinsicWidths{}, ErrIllegalArguments
	}
	if prepared.hyphenation != NoHyphenation && prepared.hyphenation != ExplicitHyphenation {
		return IntrinsicWidths{}, fmt.Errorf("%w: automatic intrinsic hyphenation", ErrUnsupportedPolicy)
	}
	// The compiler always appends this generated trailer. Validate that identity;
	// no other zero-source or zero-width knot is excluded from measurement.
	trailer := len(prepared.knots) - 1
	last, source := prepared.knots[trailer], prepared.shaping.sources[trailer]
	end := prepared.paragraph.Raw().Len()
	if last.Kind != KTGlue || last.IsDiscardable() || last.Penalty > MaxMerit || last.Len != 0 ||
		last.Pos != prepared.paragraph.Offset+end || source.atomic != 0 || source.source.byteStart != end || source.source.byteEnd != end {
		return IntrinsicWidths{}, fmt.Errorf("%w: missing prepared paragraph trailer", ErrIllegalArguments)
	}
	atoms := make(map[contracts.AtomicID]AtomicIntrinsic, len(request.Atomics))
	for _, atom := range request.Atomics {
		if err := ctx.Err(); err != nil {
			return IntrinsicWidths{}, err
		}
		_, duplicate := atoms[atom.ID]
		if atom.ID == 0 || duplicate || atom.MinAdvance <= -dimen.Fil || atom.MaxAdvance >= dimen.Fil || atom.MinAdvance > atom.MaxAdvance {
			return IntrinsicWidths{}, fmt.Errorf("%w: invalid or duplicate atomic intrinsic range", ErrIllegalArguments)
		}
		atoms[atom.ID] = atom
	}
	var rawMin, rawMax int64
	// Validate all requested identities before consulting any explicit services.
	for i := range prepared.knots {
		if err := ctx.Err(); err != nil {
			return IntrinsicWidths{}, err
		}
		if id := prepared.shaping.sources[i].atomic; id != 0 {
			if _, ok := atoms[id]; !ok {
				return IntrinsicWidths{}, fmt.Errorf("%w: missing atomic intrinsic range for %d", ErrIllegalArguments, id)
			}
		}
	}
	if prepared.hyphenation == ExplicitHyphenation {
		var err error
		rawMin, err = prepared.explicitMinContent(ctx, trailer, atoms)
		if err != nil {
			return IntrinsicWidths{}, err
		}
	}
	minStart, maxStart := 0, 0
	for i, knot := range prepared.knots {
		if err := ctx.Err(); err != nil {
			return IntrinsicWidths{}, err
		}
		limit := min(i+1, trailer) // Exclude only the validated generated trailer.
		if prepared.hyphenation == NoHyphenation && knot.Penalty < MaxPenalty {
			width, err := prepared.intrinsicSpan(ctx, minStart, limit, atoms, false)
			if err != nil {
				return IntrinsicWidths{}, err
			}
			rawMin = max(rawMin, width)
			minStart = i + 1
		}
		if knot.Penalty <= MaxMerit {
			width, err := prepared.intrinsicSpan(ctx, maxStart, limit, atoms, true)
			if err != nil {
				return IntrinsicWidths{}, err
			}
			rawMax = max(rawMax, width)
			maxStart = i + 1
		}
	}
	minimum := max(int64(0), rawMin)
	maximum := max(minimum, rawMax)
	if maximum >= int64(dimen.Fil) {
		return IntrinsicWidths{}, fmt.Errorf("%w: intrinsic width outside finite domain", ErrIllegalArguments)
	}
	if err := ctx.Err(); err != nil {
		return IntrinsicWidths{}, err
	}
	return IntrinsicWidths{MinContent: dimen.DU(minimum), MaxContent: dimen.DU(maximum)}, nil
}

// Mirror logicalLine.trimAndMeasure's first/last-content rule, but keep signed
// completed widths for intrinsic normalization. Two passes avoid assuming that
// signed prefix widths grow monotonically or rearranging additions across neutral
// retained material. Each knot is visited a bounded number of times per query.
func (p *PreparedParagraph) intrinsicSpan(ctx context.Context, start, end int, atoms map[contracts.AtomicID]AtomicIntrinsic, maximum bool) (int64, error) {
	return intrinsicKnotSpan(ctx, p.knots[start:end], func(i int) dimen.DU {
		if id := p.shaping.sources[start+i].atomic; id != 0 {
			if maximum {
				return atoms[id].MaxAdvance
			}
			return atoms[id].MinAdvance
		}
		return p.knots[start+i].W
	})
}

// advance optionally substitutes atomic endpoints without modifying the knots.
func intrinsicKnotSpan(ctx context.Context, knots []KnotCore, advance func(int) dimen.DU) (int64, error) {
	first, last := len(knots), -1
	for i, knot := range knots {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		if knot.IsContent() {
			first, last = min(first, i), i
		}
	}
	var width int64
	for i, knot := range knots {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		if knot.IsDiscardable() && (i < first || i > last) {
			continue
		}
		w := knot.W
		if advance != nil {
			w = advance(i)
		}
		var err error
		width, err = addIntrinsicAdvance(width, int64(w))
		if err != nil {
			return 0, err
		}
	}
	return width, nil
}

// Form min-content spans in logical order; post-break carry preserves the
// original after-textbox boundary. There is no working binding or fake geometry.
func (p *PreparedParagraph) explicitMinContent(ctx context.Context, trailer int, atoms map[contracts.AtomicID]AtomicIntrinsic) (int64, error) {
	var widest int64
	var span []KnotCore
	finish := func() error {
		w, err := intrinsicKnotSpan(ctx, span, nil)
		if err != nil {
			return err
		}
		widest = max(widest, w)
		span = span[:0]
		return nil
	}
	for i, knot := range p.knots[:trailer] {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		if id := p.shaping.sources[i].atomic; id != 0 {
			knot.W = atoms[id].MinAdvance
		}
		if knot.Kind == KTTextBox {
			batch, err := p.explicitReplacements(ctx, i, true)
			if err != nil {
				return 0, err
			}
			for _, replacement := range batch.replacements {
				if replacement.penalty >= MaxPenalty {
					continue
				}
				span = append(span, KnotCore{Kind: KTTextBox, W: replacement.pair.pre.widths.W})
				if err := finish(); err != nil {
					return 0, err
				}
				knot = KnotCore{Kind: KTTextBox, W: replacement.pair.post.widths.W, Penalty: knot.Penalty}
			}
		}
		span = append(span, knot)
		if knot.Penalty < MaxPenalty {
			if err := finish(); err != nil {
				return 0, err
			}
		}
	}
	if err := finish(); err != nil {
		return 0, err
	}
	return widest, nil
}

func addIntrinsicAdvance(sum, advance int64) (int64, error) {
	if (advance > 0 && sum > math.MaxInt64-advance) || (advance < 0 && sum < math.MinInt64-advance) {
		return 0, fmt.Errorf("%w: intrinsic advance accumulation overflow", ErrIllegalArguments)
	}
	return sum + advance, nil
}
