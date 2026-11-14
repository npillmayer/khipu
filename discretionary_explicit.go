package khipu

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/npillmayer/khipu/contracts"
)

// Unicode defines `U+00AD` as soft hyphens
const softHyphenMarker = "\u00ad"

// preparedDiscretionary is ready for the common fragment-shaping stage. It
// carries a whole-text replacement pair, never an independently shaped hyphen.
type preparedDiscretionary struct {
	input   discretionaryInput
	variant uint16
	penalty Penalty
}

// prepareExplicitDiscretionaries interprets markers within a single source
// textbox/context. Its caller must establish that scope; it never concatenates
// adjacent styled boxes. Only the marker is sent to the Hyphenator as a material
// request. For each opportunity the owner attaches that material to the original
// left/right text, then uses the same provenance contract as automatic discovery.
// No shaping, cache publication or no-break paragraph rewriting occurs here.
//
// All U+00AD markers disappear from replacement input, including unselected
// markers. The selected source split is AFTER its marker, whose original bytes
// and scalar remain in pre-break provenance. Markers without text on both sides
// are ignored. This is not a general word/fragment-length or script-validity rule.
// Variant IDs are deterministic in source order then material order; callers
// must not merge this namespace with independently numbered automatic variants.
func (kq *Khipukamayuq) prepareExplicitDiscretionaries(original string, source runSourceRange) ([]preparedDiscretionary, error) {
	return kq.prepareExplicitDiscretionariesContext(context.Background(), original, source)
}

func (kq *Khipukamayuq) prepareExplicitDiscretionariesContext(ctx context.Context, original string, source runSourceRange) ([]preparedDiscretionary, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if kq == nil {
		return nil, ErrVoidKhipukamayuq
	}
	// Validate source coordinates even if no policy or marker is present.
	if _, err := prepareDiscretionaryInput(original, source, HyphenationFragment{SourceSplit: SourceSplit{0, true}}); err != nil {
		return nil, err
	}
	if kq.env.hyphenator == nil || !strings.Contains(original, softHyphenMarker) {
		return nil, nil
	}
	type halves struct {
		left, right string
		split       uint64
	}
	var opportunities []halves
	positions, err := explicitSplitPositions(ctx, original)
	if err != nil {
		return nil, err
	}
	for _, pos := range positions {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		end := pos + len(softHyphenMarker)
		left := strings.ReplaceAll(original[:pos], softHyphenMarker, "")
		right := strings.ReplaceAll(original[end:], softHyphenMarker, "")
		if left != "" && right != "" {
			opportunities = append(opportunities, halves{left, right, uint64(end)})
		}
	}
	if len(opportunities) == 0 {
		return nil, nil
	}
	policy := *kq.env.params
	materials, err := kq.env.hyphenator.Discretionaries(softHyphenMarker, &policy)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr
	}
	if err != nil {
		return nil, err
	}
	if len(materials) == 0 {
		return nil, nil
	}
	materials = slices.Clone(materials)
	if len(opportunities) > math.MaxUint16/len(materials) {
		return nil, fmt.Errorf("%w: too many explicit discretionary variants", contracts.ErrIllegalArguments)
	}
	for _, m := range materials {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !utf8.ValidString(m.PreBreak) || !utf8.ValidString(m.PostBreak) ||
			strings.Contains(m.PreBreak, softHyphenMarker) || strings.Contains(m.PostBreak, softHyphenMarker) {
			return nil, fmt.Errorf("%w: invalid explicit break material", contracts.ErrIllegalArguments)
		}
	}
	result := make([]preparedDiscretionary, 0, len(opportunities)*len(materials))
	for _, op := range opportunities {
		for _, m := range materials {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			fragment := HyphenationFragment{PreBreak: op.left + m.PreBreak, PostBreak: m.PostBreak + op.right, SourceSplit: SourceSplit{op.split, true}}
			input, err := prepareDiscretionaryInput(original, source, fragment)
			if err != nil {
				return nil, err
			}
			result = append(result, preparedDiscretionary{input: input, variant: uint16(len(result) + 1), penalty: discretionaryPenalty(m, kq.env.params)})
		}
	}
	return result, nil
}

// Count source positions, not distinct replacement spellings. Consecutive
// interior markers remain separate positions; edge-only markers are ignored.
func explicitSplitPositions(ctx context.Context, original string) ([]int, error) {
	first, last := len(original), -1
	var markers []int
	for pos, r := range original {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if r == '\u00ad' {
			markers = append(markers, pos)
		} else {
			first, last = min(first, pos), pos
		}
	}
	positions := markers[:0]
	for _, pos := range markers {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if first < pos && pos < last {
			positions = append(positions, pos)
		}
	}
	return positions, nil
}
