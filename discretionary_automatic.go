package khipu

import (
	"fmt"
	"math"
	"strings"

	"github.com/npillmayer/khipu/contracts"
)

// prepareAutomaticDiscretionaries is called only after structural eligibility.
// The policy receives the complete original segment and a local parameter copy
// containing its resolved language/script/direction. It returns complete
// replacement strings; unlike marker material, nothing is concatenated here.
//
// Validate the entire response before shaping. A split must be known, interior
// to original input and on a UTF-8 scalar boundary, independently of replacement
// lengths. Script-specific break safety remains the policy's responsibility.
// Empty halves are skipped; no rune-count minimum is
// imposed. Recursive soft-hyphen material is rejected. Nonzero policy variant
// IDs are retained; zero IDs receive the lowest free positive IDs in response
// order. Duplicates/overflow fail rather than corrupting breakpoint identity.
func (kq *Khipukamayuq) prepareAutomaticDiscretionaries(original string, source runSourceRange, params Params) ([]preparedDiscretionary, error) {
	fragments, err := kq.env.hyphenator.Discretionaries(original, &params)
	if err != nil {
		return nil, err
	}
	bad := func(reason string) ([]preparedDiscretionary, error) {
		return nil, fmt.Errorf("%w: automatic discretionary: %s", contracts.ErrIllegalArguments, reason)
	}
	if len(fragments) > math.MaxUint16 {
		return bad("too many variants")
	}
	used := make(map[uint16]bool)
	result := make([]preparedDiscretionary, 0, len(fragments))
	for _, f := range fragments {
		input, err := prepareDiscretionaryInput(original, source, f)
		if err != nil {
			return nil, err
		}
		if f.SourceSplit.ByteOffset == 0 || f.SourceSplit.ByteOffset == uint64(len(original)) {
			return bad("split must be interior")
		}
		if strings.Contains(f.PreBreak, softHyphenMarker) || strings.Contains(f.PostBreak, softHyphenMarker) {
			return bad("recursive marker in replacement")
		}
		if f.PreBreak == "" || f.PostBreak == "" {
			continue
		}
		if f.Variant != 0 {
			if used[f.Variant] {
				return bad("duplicate variant")
			}
			used[f.Variant] = true
		}
		result = append(result, preparedDiscretionary{input: input, variant: f.Variant, penalty: discretionaryPenalty(f, &params)})
	}
	next := uint32(1)
	for i := range result {
		if result[i].variant != 0 {
			continue
		}
		for used[uint16(next)] {
			next++
		}
		// len(fragments) bounds the number of occupied and requested IDs.
		result[i].variant = uint16(next)
		used[uint16(next)] = true
	}
	return result, nil
}
