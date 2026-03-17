package khipu

// HyphenationFragment is one hyphenation alternative for a textbox.
// The Khipukamayuq will shape these fragments and turn them into
// discretionary candidates stored on a Khipu.
type HyphenationFragment struct {
	Variant   uint16
	PreBreak  string
	PostBreak string
	Penalty   Penalty
}

// Hyphenator discovers hyphenation alternatives for one textbox string.
// It returns fragment strings; shaping and final discretionary construction
// remain the responsibility of Khipukamayuq.
type Hyphenator interface {
	Discretionaries(word string, params *Params) ([]HyphenationFragment, error)
}
