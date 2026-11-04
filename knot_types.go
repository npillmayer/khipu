package khipu

// KnotType identifies the kind of a knot in either paragraph representation.
type KnotType int8

// Knot kinds retain their existing numeric values during the migration.
// KTPenalty and KTDiscretionary remain until their legacy consumers and the
// remaining modern fixtures are migrated.
const (
	KTKern KnotType = iota
	KTGlue
	KTTextBox
	KTPenalty
	KTDiscretionary
	KTUserDefined // clients should use custom knot types above this
)
