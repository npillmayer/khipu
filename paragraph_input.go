package khipu

import (
	"errors"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
)

// ErrUnsupportedPolicy identifies recognized semantics that cannot be realized
// by the current pipeline. It is distinct from malformed arguments.
var ErrUnsupportedPolicy = errors.New("khipu: unsupported policy")

// ParagraphInput is normalized styled text with opaque inline occurrences.
// Every anchor selects one U+FFFC; unanchored markers remain ordinary text.
// Text, styles and external font services are borrowed and must remain stable
// for all prepared, bound and frozen results derived from this input.
type ParagraphInput struct {
	Paragraph *styled.Paragraph
	Atomics   []contracts.AtomicAnchor
}

// BoundaryAction permits an ordinary zero-penalty break or prohibits an optional
// one. Zero and unknown values are invalid; mandatory overrides are unsupported.
type BoundaryAction uint8

const (
	// AllowBoundary permits an optional zero-penalty break at the atomic edge.
	AllowBoundary BoundaryAction = iota + 1
	// ProhibitBoundary prohibits an optional break at the atomic edge.
	ProhibitBoundary
)

// AtomicBoundary overrides one interior boundary immediately before or after
// an attached atom. Offsets are paragraph-relative bytes, sorted and unique.
// Omission retains Unicode policy. Mandatory breaks cannot be downgraded.
type AtomicBoundary struct {
	ByteOffset uint64
	Action     BoundaryAction
}

// HyphenationMode fixes discovery policy for all bindings of a preparation.
// Enabled modes require a configured Hyphenator; NoHyphenation is the default.
type HyphenationMode uint8

const (
	// NoHyphenation disables explicit and automatic discretionary discovery.
	NoHyphenation HyphenationMode = iota
	// ExplicitHyphenation permits authored soft-hyphen replacements only.
	ExplicitHyphenation
	AutomaticHyphenation // Includes explicit opportunities.
)

// PrepareOptions supplies paragraph-wide policies, without target geometry.
type PrepareOptions struct {
	AtomicBoundaries []AtomicBoundary
	Hyphenation      HyphenationMode
}

// AtomicAdvance supplies a fixed signed inline contribution, including any
// caller-resolved margins. Its finite magnitude must be less than dimen.Fil.
type AtomicAdvance struct {
	ID      contracts.AtomicID
	Advance dimen.DU
}
