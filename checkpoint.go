package khipu

import (
	"fmt"
	"slices"
)

// BreakCheckpoint is an immutable, preparation-bound prefix of a complete legal
// ordinary break path. It contains logical knot boundaries, never source-byte
// offsets or frozen visual indexes. Its zero value is invalid.
//
// A checkpoint keeps its preparation and owner alive. Drop checkpoints, bindings
// and preparation when the paragraph has fully flowed (or is abandoned); frozen
// results do not retain them. Serialize operations on the owner and keep borrowed
// source/styles/services stable. Checkpoints are not serialization records.
type BreakCheckpoint struct {
	prepared *PreparedParagraph
	prefix   []int
}

// Checkpoint certifies the first acceptedLines of a successful complete break
// path against prepared's immutable ordinary boundary policy. Pass the complete
// path returned by a breaker, including the paragraph-final trailer, not merely
// its prefix. Zero accepted lines and acceptance of the entire path are valid.
// A later successful continuation path may certify a longer prefix in the same way.
//
// Certification checks every boundary and mandatory break, including the unaccepted
// suffix. It establishes path legality, not width fit, optimality or which breaker
// produced it; first-fit and KP paths are equally admissible. No binding, shaping,
// font acceptance, metric query or bidi analysis is performed. Only NoHyphenation
// preparations are supported: discretionary replacement carry is not captured.
// The prefix is copied; no working arrays or caller slices are retained.
// Cancellation/errors return no checkpoint and preserve earlier results.
func (q *Khipukamayuq) Checkpoint(prepared *PreparedParagraph, path []int, acceptedLines int) (*BreakCheckpoint, error) {
	if q == nil || prepared == nil || prepared.owner != q || prepared.preparedMaterial == nil {
		return nil, ErrIllegalArguments
	}
	if prepared.hyphenation != NoHyphenation {
		return nil, ErrUnsupportedPolicy
	}
	n := len(prepared.knots)
	if n == 0 || len(path) == 0 || path[len(path)-1] != n-1 || acceptedLines < 0 || acceptedLines > len(path) {
		return nil, ErrIllegalArguments
	}
	previous := -1
	for _, at := range path {
		if at <= previous || at >= n || prepared.knots[at].Penalty >= MaxPenalty {
			return nil, fmt.Errorf("%w: invalid checkpoint boundary", ErrIllegalArguments)
		}
		for i := previous + 1; i < at; i++ {
			if prepared.knots[i].Penalty <= MaxMerit {
				return nil, fmt.Errorf("%w: checkpoint path skips mandatory break", ErrIllegalArguments)
			}
		}
		previous = at
	}
	c := &BreakCheckpoint{prepared: prepared, prefix: slices.Clone(path[:acceptedLines])}

	return c, nil
}

// PrefixFor returns detached certified boundaries for a binding of this exact
// preparation. It is the breaker-facing checkpoint seam: ownership, policy and
// plane lengths are checked without scanning prefix knots. Base planes must stay
// read-only, as for all owner-produced Khipus. Modifying the returned slice cannot
// change the checkpoint. Nil/zero checkpoints or foreign/uninitialized bindings
// return ErrIllegalArguments and no prefix.
func (c *BreakCheckpoint) PrefixFor(k *Khipu) ([]int, error) {
	if c == nil || c.prepared == nil || c.prepared.owner == nil || c.prepared.preparedMaterial == nil ||
		k == nil || k.owner != c.prepared.owner || k.boundMaterial != c.prepared.preparedMaterial || k.paragraph != c.prepared.paragraph {
		return nil, ErrIllegalArguments
	}
	if c.prepared.hyphenation != NoHyphenation || !k.hyphenationPolicy || k.hyphenation != NoHyphenation {
		return nil, ErrUnsupportedPolicy
	}
	n := len(c.prepared.knots)
	for _, length := range []int{len(k.Kind), len(k.W), len(k.MinW), len(k.MaxW), len(k.Penalty), len(k.Pos), len(k.Len), len(k.Flags), len(k.boxRefs), len(k.shaping.sources)} {
		if length != n {
			return nil, ErrIllegalArguments
		}
	}
	prefix := slices.Clone(c.prefix)

	return prefix, nil
}
