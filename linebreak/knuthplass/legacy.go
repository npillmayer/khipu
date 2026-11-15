package knuthplass

import "fmt"

// BreakRef identifies a logical breakpoint. At is a zero-based knot index;
// Variant zero denotes an ordinary break, and a positive Variant identifies
// a discretionary alternative inside that knot. Negative At denotes the start.
//
// Deprecated: this search record is retained for compatibility. Clients should
// use the indices returned by BreakParagraph together with the input Khipu's
// SelectedDiscretionaries, or query the resulting FrozenKhipu after freezing.
type BreakRef struct {
	At      int
	Variant uint16
}

// NoBreakRef is the legacy missing-break value, initially {At: -1, Variant: 0}.
// The breaker does not read this variable; assigning it cannot configure search.
//
// Deprecated: use BreakParagraph's returned indices and error. Existing users
// needing a sentinel should keep their own BreakRef{At: -1} value.
var NoBreakRef = BreakRef{At: noinx}

// PlainBreak constructs an ordinary BreakRef at the given zero-based knot index.
// A negative index denotes the start rather than a source knot.
//
// Deprecated: use the indices returned by BreakParagraph directly.
func PlainBreak(at int) BreakRef {
	return BreakRef{At: at}
}

// IsDiscretionary reports whether the legacy record has a positive Variant.
//
// Deprecated: query the input Khipu's SelectedDiscretionaries for chosen variants.
func (br BreakRef) IsDiscretionary() bool {
	return br.Variant > 0
}

// Kinx returns At, the zero-based knot index (negative for the start sentinel).
//
// Deprecated: use BreakRef.At when maintaining legacy records, or use the
// indices returned by BreakParagraph in new code.
func (br BreakRef) Kinx() int {
	return br.At
}

// String formats the legacy record as START, k-N, or k-N[v:V].
func (br BreakRef) String() string {
	if br.At < 0 {
		return "START"
	}
	if br.Variant == 0 {
		return fmt.Sprintf("k-%d", br.At)
	}
	return fmt.Sprintf("k-%d[v:%d]", br.At, br.Variant)
}

// LICContent is the legacy internal classification for visible content.
//
// Deprecated: use khipu.KnotCore.IsContent to inspect a knot.
const LICContent = licContent

// LICTrimDiscardable is the legacy internal classification for edge trimming.
//
// Deprecated: use khipu.KnotCore.IsDiscardable to inspect a knot.
const LICTrimDiscardable = licTrimDiscardable

// LICRetainedNeutral is the legacy internal classification for retained material
// that is neither discardable nor visible content, such as paragraph fill.
//
// Deprecated: inspect khipu.KnotCore.IsDiscardable and IsContent instead.
const LICRetainedNeutral = licRetainedNeutral
