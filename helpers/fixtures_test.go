package helpers_test

import "github.com/npillmayer/cords/styled"

type runeReadFunc func() (rune, int, error)

func (f runeReadFunc) ReadRune() (rune, int, error) { return f() }

// Explicit expectations also cover controls which deliberately remain cells.
var syntheticCells = []struct {
	text string
	wide bool
}{
	{"\n", false}, {"\r", false}, {"\v", false}, {"\f", false},
	{"\u0085", false}, {"\u2028", false}, {"\u2029", false},
	{"\u200b", false}, {"\u2060", false},
	{"a", true}, {"é", true}, {"\U0001f600", true}, {"\ufffd", true},
	{" ", true}, {"\u00a0", true}, {"\t", true}, {"\x00", true},
	{"\u00ad", true}, {"\u0301", true}, {"\u200c", true},
	{"\u200d", true}, {"\ufe0f", true}, {"\u2067", true},
}

type testStyle string

func (s testStyle) Equals(other styled.Style) bool { return s == other }
func (s testStyle) String() string                 { return string(s) }
