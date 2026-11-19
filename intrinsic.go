package khipu

import (
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
// Discovery must match the material's incarnation even with NoHyphenation.
// Inputs are consumed synchronously; failures return zero widths. Complete cached
// batches survive failed trials, and intrinsic rejection never poisons layout's
// cache. Borrowed source/styles/services must remain stable.
func MeasureIntrinsic(prepared *Khipu, discovery *Discovery, request MeasureRequest) (IntrinsicWidths, error) {
	if !discovery.validFor(prepared) || prepared.KnotCount() == 0 || prepared.material.paragraph == nil || prepared.KnotCount() != len(prepared.material.shaping.sources) || len(request.Atomics) != len(prepared.atomicIDs) {
		return IntrinsicWidths{}, ErrIllegalArguments
	}
	if prepared.hyphenation != NoHyphenation && prepared.hyphenation != ExplicitHyphenation {
		return IntrinsicWidths{}, fmt.Errorf("%w: automatic intrinsic hyphenation", ErrUnsupportedPolicy)
	}
	// The compiler always appends this generated trailer. Validate that identity;
	// no other zero-source or zero-width knot is excluded from measurement.
	trailer := prepared.KnotCount() - 1
	last, source := prepared.material.planes.knot(trailer), prepared.material.shaping.sources[trailer]
	end := prepared.material.paragraph.Raw().Len()
	if last.Kind != KTGlue || last.IsDiscardable() || last.Penalty > MaxMerit || last.Len != 0 ||
		last.Pos != prepared.material.paragraph.Offset+end || source.atomic != 0 || source.source.byteStart != end || source.source.byteEnd != end {
		return IntrinsicWidths{}, fmt.Errorf("%w: missing prepared paragraph trailer", ErrIllegalArguments)
	}
	atoms := make(map[contracts.AtomicID]AtomicIntrinsic, len(request.Atomics))
	for _, atom := range request.Atomics {
		_, duplicate := atoms[atom.ID]
		if atom.ID == 0 || duplicate || atom.MinAdvance <= -dimen.Fil || atom.MaxAdvance >= dimen.Fil || atom.MinAdvance > atom.MaxAdvance {
			return IntrinsicWidths{}, fmt.Errorf("%w: invalid or duplicate atomic intrinsic range", ErrIllegalArguments)
		}
		atoms[atom.ID] = atom
	}
	var rawMin, rawMax int64
	// Validate all requested identities before consulting any explicit services.
	for i := 0; i < prepared.KnotCount(); i++ {
		if id := prepared.material.shaping.sources[i].atomic; id != 0 {
			if _, ok := atoms[id]; !ok {
				return IntrinsicWidths{}, fmt.Errorf("%w: missing atomic intrinsic range for %d", ErrIllegalArguments, id)
			}
		}
	}
	if prepared.hyphenation == ExplicitHyphenation {
		var err error
		rawMin, err = prepared.explicitMinContent(discovery, trailer, atoms)
		if err != nil {
			return IntrinsicWidths{}, err
		}
	}
	minStart, maxStart := 0, 0
	for i := 0; i < prepared.KnotCount(); i++ {
		knot := prepared.material.planes.knot(i)
		limit := min(i+1, trailer) // Exclude only the validated generated trailer.
		if prepared.hyphenation == NoHyphenation && knot.Penalty < MaxPenalty {
			width, err := prepared.intrinsicSpan(minStart, limit, atoms, false)
			if err != nil {
				return IntrinsicWidths{}, err
			}
			rawMin = max(rawMin, width)
			minStart = i + 1
		}
		if knot.Penalty <= MaxMerit {
			width, err := prepared.intrinsicSpan(maxStart, limit, atoms, true)
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

	return IntrinsicWidths{MinContent: dimen.DU(minimum), MaxContent: dimen.DU(maximum)}, nil
}

// Mirror logicalLine.trimAndMeasure's first/last-content rule, but keep signed
// completed widths for intrinsic normalization. Two passes avoid assuming that
// signed prefix widths grow monotonically or rearranging additions across neutral
// retained material. Each knot is visited a bounded number of times per query.
func (p *Khipu) intrinsicSpan(start, end int, atoms map[contracts.AtomicID]AtomicIntrinsic, maximum bool) (int64, error) {
	knots := make([]KnotCore, end-start)
	for i := range knots {
		knots[i] = p.material.planes.knot(start + i)
	}
	return intrinsicKnotSpan(knots, func(i int) dimen.DU {
		if id := p.material.shaping.sources[start+i].atomic; id != 0 {
			if maximum {
				return atoms[id].MaxAdvance
			}
			return atoms[id].MinAdvance
		}
		return p.material.planes.w[start+i]
	})
}

// advance optionally substitutes atomic endpoints without modifying the knots.
func intrinsicKnotSpan(knots []KnotCore, advance func(int) dimen.DU) (int64, error) {
	first, last := len(knots), -1
	for i, knot := range knots {
		if knot.IsContent() {
			first, last = min(first, i), i
		}
	}
	var width int64
	for i, knot := range knots {
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
func (p *Khipu) explicitMinContent(discovery *Discovery, trailer int, atoms map[contracts.AtomicID]AtomicIntrinsic) (int64, error) {
	var widest int64
	var span []KnotCore
	finish := func() error {
		w, err := intrinsicKnotSpan(span, nil)
		if err != nil {
			return err
		}
		widest = max(widest, w)
		span = span[:0]
		return nil
	}
	for i := 0; i < trailer; i++ {
		knot := p.material.planes.knot(i)
		if id := p.material.shaping.sources[i].atomic; id != 0 {
			knot.W = atoms[id].MinAdvance
		}
		if knot.Kind == KTTextBox {
			batch, err := discovery.alternatives(i, true)
			if err != nil {
				return 0, err
			}
			for _, replacement := range batch.alternatives {
				if replacement.preCore.Penalty >= MaxPenalty {
					continue
				}
				span = append(span, KnotCore{Kind: KTTextBox, W: replacement.preCore.W})
				if err := finish(); err != nil {
					return 0, err
				}
				knot = KnotCore{Kind: KTTextBox, W: replacement.postCore.W, Penalty: knot.Penalty}
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
