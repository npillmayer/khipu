package khipu

import (
	"fmt"

	"github.com/npillmayer/khipu/dimen"
)

// KnotType identifies the kind of a knot in the SOA paragraph.
type KnotType int8

// Supported kinds retain their numeric values. Value 4 is reserved:
// penalties are fields on knots, and discretionary alternatives live in sparse
// candidate/selection tables rather than standalone knots.
const (
	KTKern        KnotType = 0
	KTGlue        KnotType = 1
	KTTextBox     KnotType = 2
	KTAtomic      KnotType = 3 // fixed-size opaque inline content, never shaped
	KTUserDefined KnotType = 5 // clients should use custom knot types above this
)

// KnotFlags records knot metadata independently of kind and break penalty.
type KnotFlags uint8

const (
	// KFDiscardable permits trimming at line edges. It does not authorize a
	// line break. Generated paragraph fill and atomic content are not discardable.
	KFDiscardable KnotFlags = 1 << iota
)

// KnotCore is a value snapshot of one elastic knot's metrics and source span.
// Widths are in dimen.DU. Fixed material, including signed atomic advances,
// has MinW == W == MaxW; MaxW may instead hold a fill sentinel for glue.
// Finite MaxW describes stretch capacity; freezing may exceed it and report
// an underfull outcome. Shrink remains bounded by MinW.
// The generated paragraph trailer has an empty source span and a forced break.
type KnotCore struct {
	// Pos is the absolute UTF-8 byte position, including the paragraph offset.
	Pos uint64
	// W, MinW and MaxW are natural, minimum and maximum widths, not deltas.
	W, MinW, MaxW dimen.DU
	// Len counts original source bytes, not glyphs or replacement bytes.
	// A single knot can represent at most 65535 source bytes.
	Len uint16
	// Penalty applies to the boundary after this knot.
	Penalty Penalty
	// Kind identifies the material; Flags independently controls discardability.
	Kind  KnotType
	Flags KnotFlags
}

// IsContent reports material that shelters adjacent spaces from line-edge
// trimming. Atomic content counts even when its advance is zero or negative.
func (k KnotCore) IsContent() bool {
	return !k.IsDiscardable() && (k.Kind == KTTextBox || k.Kind == KTAtomic)
}

// String returns a compact diagnostic description of the knot's kind and width.
func (k KnotCore) String() string {
	switch k.Kind {
	case KTGlue:
		return fmt.Sprintf("\u25cb%.2f\u00b1[p=%d]", k.W.Points(), k.Penalty)
	case KTTextBox:
		return fmt.Sprintf("\u00ab%s\u00bb(%.2f)[p=%d]", "text", k.W.Points(), k.Penalty)
	}
	return fmt.Sprintf("%d:%.2f", k.Kind, k.W.Points())
}

func defaultFlagsForKind(kind KnotType) KnotFlags {
	switch kind {
	case KTGlue, KTKern:
		return KFDiscardable
	default:
		return 0
	}
}

// IsDiscardable reports whether KFDiscardable is set, regardless of knot kind
// or penalty. [KnotCore.IsContent] determines which retained items shelter spaces.
func (k KnotCore) IsDiscardable() bool {
	return k.Flags&KFDiscardable != 0
}
