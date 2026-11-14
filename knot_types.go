package khipu

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
