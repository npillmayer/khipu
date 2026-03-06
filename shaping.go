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

type Shaper interface {
	SetSource(RuneReader)
	SetStyle(styled.BidiStyleChange)
	ReadClusterMetrics() (clusterId int, width dimen.DU, textPos int, err error)
}

// ---------------------------------------------------------------------------
type fixedShaper struct {
	source       RuneReader
	clusterCount int
}

func newFixedShaper() *fixedShaper {
	return &fixedShaper{}
}

func (sh *fixedShaper) SetSource(source RuneReader) {
	sh.source = source
}

func (sh *fixedShaper) SetStyle(styleChange styled.BidiStyleChange) {
}

func (sh *fixedShaper) SetReaderSource(r io.Reader) {
	sh.source = bufio.NewReader(r)
}

func (sh *fixedShaper) ReadClusterMetrics() (clusterId int, width dimen.DU, len int, err error) {
	if sh.source == nil {
		return 0, 0, 0, io.EOF
	}
	var r rune = '\ufffd' // invalid rune signal by RuneReader.ReadRune(…)
	for r == '\ufffd' {
		r, len, err = sh.source.ReadRune()
		if len == 0 {
			r = '\ufffd' // skip and continue
		}
	}
	cluster := sh.clusterCount
	sh.clusterCount += 1
	return cluster, 5 * dimen.PT, len, err
}

var _ Shaper = (*fixedShaper)(nil)
