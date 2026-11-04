package khipu

import (
	"bufio"
	"io"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/dimen"
)

type RuneReader interface {
	ReadRune() (rune, int, error)
}

// Shaper supplies metrics in logical source order. SetSource starts a new run
// and resets its byte offsets. A metric's clusterPos is the run-relative UTF-8
// byte offset of its cluster's first rune, not a sequential cluster number.
// Repeated offsets accumulate width; offsets must not decrease. Zero-width
// metrics still mark cluster starts. EOF carries no metric and all other errors
// abort encoding. sourceLen is informational; cluster positions define spans.
type Shaper interface {
	SetSource(RuneReader)
	SetStyle(styled.BidiStyleChange)
	ReadClusterMetrics() (clusterPos int, width dimen.DU, sourceLen int, err error)
}

// ---------------------------------------------------------------------------
type fixedShaper struct {
	source     RuneReader
	byteOffset int
}

func newFixedShaper() *fixedShaper {
	return &fixedShaper{}
}

func (sh *fixedShaper) SetSource(source RuneReader) {
	sh.source = source
	sh.byteOffset = 0
}

func (sh *fixedShaper) SetStyle(styleChange styled.BidiStyleChange) {
}

func (sh *fixedShaper) SetReaderSource(r io.Reader) {
	sh.SetSource(bufio.NewReader(r))
}

func (sh *fixedShaper) ReadClusterMetrics() (clusterId int, width dimen.DU, len int, err error) {
	if sh.source == nil {
		return 0, 0, 0, io.EOF
	}
	r, size, err := sh.source.ReadRune()
	if err != nil {
		return 0, 0, 0, err
	}
	cluster := sh.byteOffset
	sh.byteOffset += size
	w := 5 * dimen.PT
	if r == '\n' || r == '\r' || r == '\v' || r == '\f' || r == '\u0085' || r == '\u2028' || r == '\u2029' || r == '\u200b' || r == '\u2060' {
		w = 0
	}
	return cluster, w, size, nil
}

var _ Shaper = (*fixedShaper)(nil)
