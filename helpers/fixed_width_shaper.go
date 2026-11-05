package helpers

import (
	"bufio"
	"fmt"
	"io"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
)

// FixedWidthShaper provides deterministic metrics, one per rune in source order.
// Its zero value uses a cell width of 5*dimen.PT. NewFixedWidthShaper selects a
// different positive width. LF, CR, VT, FF, NEL, LS, PS, U+200B, and U+2060 have
// zero width. All other runes, including tabs and combining marks, use one cell.
// It does no normalization, shaping, or visual reordering and ignores styles.
//
// Use it with khipu.Config.Shaper. It produces DU metrics without a FontFace.
// Instances are mutable: do not copy them during use or share them concurrently.
type FixedWidthShaper struct {
	source     contracts.RuneReader
	byteOffset int
	cellWidth  dimen.DU
}

// NewFixedWidthShaper creates a metric shaper with a positive cell width in DU.
// Nonpositive widths return an error wrapping contracts.ErrIllegalArguments.
// To use the default width without a constructor, use &FixedWidthShaper{}.
func NewFixedWidthShaper(cellWidth dimen.DU) (*FixedWidthShaper, error) {
	if cellWidth <= 0 {
		return nil, fmt.Errorf("%w: fixed cell width must be positive", contracts.ErrIllegalArguments)
	}
	return &FixedWidthShaper{cellWidth: cellWidth}, nil
}

// SetSource selects a run and resets its byte offsets. Nil clears the source.
func (sh *FixedWidthShaper) SetSource(source contracts.RuneReader) {
	sh.source = source
	sh.byteOffset = 0
}

// SetStyle has no effect on synthetic metrics.
func (sh *FixedWidthShaper) SetStyle(styleChange styled.BidiStyleChange) {
}

// SetReaderSource wraps a byte reader as a rune source. Nil clears the source.
func (sh *FixedWidthShaper) SetReaderSource(r io.Reader) {
	if r == nil {
		sh.SetSource(nil)
		return
	}
	sh.SetSource(bufio.NewReader(r))
}

// ReadClusterMetrics returns the run-relative byte offset, cell width, and
// byte length of the next rune. EOF and other reader errors carry no metric.
// The encoder validates UTF-8; this metric helper delegates reading to its source.
func (sh *FixedWidthShaper) ReadClusterMetrics() (clusterPos int, width dimen.DU, sourceLen int, err error) {
	if sh.source == nil {
		return 0, 0, 0, io.EOF
	}
	r, size, err := sh.source.ReadRune()
	if err != nil {
		return 0, 0, 0, err
	}
	cluster := sh.byteOffset
	sh.byteOffset += size
	w := sh.cellWidth
	if w == 0 {
		w = 5 * dimen.PT
	}
	if syntheticZeroWidth(r) {
		w = 0
	}
	return cluster, w, size, nil
}

func syntheticZeroWidth(r rune) bool {
	switch r {
	case '\n', '\r', '\v', '\f', '\u0085', '\u2028', '\u2029', '\u200b', '\u2060':
		return true
	}
	return false
}

var _ contracts.Shaper = (*FixedWidthShaper)(nil)
