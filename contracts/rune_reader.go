package contracts

// RuneReader supplies decoded Unicode scalars to the streaming shaper.
type RuneReader interface {
	ReadRune() (rune, int, error)
}
