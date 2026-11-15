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

func (khipu *Khipu) flagsAt(index int) KnotFlags {
	if khipu == nil || index < 0 || index >= len(khipu.Flags) {
		return 0
	}
	return khipu.Flags[index]
}

// IsDiscardable reports whether KFDiscardable is set, regardless of knot kind
// or penalty. [KnotCore.IsContent] determines which retained items shelter spaces.
func (k KnotCore) IsDiscardable() bool {
	return k.Flags&KFDiscardable != 0
}

// SetKnotFlags replaces the flags at index, padding missing flag entries with
// zero. It returns false for a nil receiver or an index outside the Kind plane.
// It changes trimming metadata, not break penalties or retained provenance.
func (khipu *Khipu) SetKnotFlags(index int, flags KnotFlags) bool {
	if khipu == nil || index < 0 || index >= len(khipu.Kind) {
		return false
	}
	if len(khipu.Flags) < len(khipu.Kind) {
		missing := len(khipu.Kind) - len(khipu.Flags)
		khipu.Flags = append(khipu.Flags, make([]KnotFlags, missing)...)
	}
	khipu.Flags[index] = flags
	return true
}

// Discardable scans backward from index, inclusively, past flagged discardables.
// It returns the first non-discardable index and whether anything was skipped.
// If every knot through index is discardable it returns (0, true); an invalid
// index returns (0, false). The boolean is not a found indicator. The receiver
// must be nonnil. Missing flag entries are treated as non-discardable.
func (khipu *Khipu) Discardable(index int) (int, bool) {
	if index < 0 || index >= len(khipu.Kind) {
		return 0, false
	}
	discarded := false
	for i := index; i >= 0; i-- {
		if khipu.flagsAt(i)&KFDiscardable == 0 {
			return i, discarded
		}
		discarded = true
	}
	return 0, discarded
}
