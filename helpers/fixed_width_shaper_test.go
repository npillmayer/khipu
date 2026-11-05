package helpers_test

import (
	"errors"
	"io"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
)

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

func TestFixedWidthShaperCellsAndByteOffsets(t *testing.T) {
	custom, err := helpers.NewFixedWidthShaper(7 * dimen.PT)
	if err != nil {
		t.Fatal(err)
	}
	for _, implementation := range []struct {
		shaper *helpers.FixedWidthShaper
		width  dimen.DU
	}{{&helpers.FixedWidthShaper{}, 5 * dimen.PT}, {custom, 7 * dimen.PT}} {
		var text strings.Builder
		for _, cell := range syntheticCells {
			text.WriteString(cell.text)
		}
		for range 2 {
			implementation.shaper.SetReaderSource(strings.NewReader(text.String()))
			implementation.shaper.SetStyle(styled.BidiStyleChange{Style: testStyle("ignored")})
			offset := 0
			for _, cell := range syntheticCells {
				at, width, size, err := implementation.shaper.ReadClusterMetrics()
				want := dimen.DU(0)
				if cell.wide {
					want = implementation.width
				}
				if err != nil || at != offset || size != len(cell.text) || width != want {
					t.Fatalf("%q at %d: got (%d, %d, %d, %v), want width %d", cell.text, offset, at, width, size, err, want)
				}
				offset += len(cell.text)
			}
			for range 2 {
				at, width, size, err := implementation.shaper.ReadClusterMetrics()
				if err != io.EOF || at != 0 || width != 0 || size != 0 {
					t.Fatalf("EOF carried a metric: %d %d %d %v", at, width, size, err)
				}
			}
		}
	}
}

func TestFixedWidthShaperErrorsAndReuse(t *testing.T) {
	for _, width := range []dimen.DU{0, -1} {
		if sh, err := helpers.NewFixedWidthShaper(width); sh != nil || !errors.Is(err, contracts.ErrIllegalArguments) {
			t.Fatalf("invalid width accepted: %v %v", sh, err)
		}
	}
	sh := &helpers.FixedWidthShaper{}
	wantErr := errors.New("reader failed")
	sh.SetSource(runeReadFunc(func() (rune, int, error) { return 'x', 1, wantErr }))
	if at, width, size, err := sh.ReadClusterMetrics(); err != wantErr || at != 0 || width != 0 || size != 0 {
		t.Fatalf("reader failure carried a metric: %d %d %d %v", at, width, size, err)
	}
	sh.SetSource(strings.NewReader("é"))
	if at, width, size, err := sh.ReadClusterMetrics(); err != nil || at != 0 || size != utf8.RuneLen('é') || width != 5*dimen.PT {
		t.Fatalf("new source did not reset shaper: %d %d %d %v", at, width, size, err)
	}
	sh.SetReaderSource(nil)
	if _, _, _, err := sh.ReadClusterMetrics(); err != io.EOF {
		t.Fatalf("nil source: %v", err)
	}
}

type testStyle string

func (s testStyle) Equals(other styled.Style) bool { return s == other }
func (s testStyle) String() string                 { return string(s) }
