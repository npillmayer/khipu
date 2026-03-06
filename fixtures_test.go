package khipu

import (
	"fmt"
	"io"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/dimen"
)

// --- Test Styles -----------------------------------------------------------

type mystyle string

func teststyle(sty string) mystyle {
	return mystyle(sty)
}

func (sty mystyle) Equals(other styled.Style) (ok bool) {
	var o mystyle
	if o, ok = other.(mystyle); !ok {
		return false
	}
	return sty == o
}

func (sty mystyle) String() string {
	return fmt.Sprintf("(style:%s)", string(sty))
}

// --- Test Shaper -----------------------------------------------------------

type testshaper struct {
	source       RuneReader
	clusterCount int
}

func (sh *testshaper) SetSource(source RuneReader) {
	sh.source = source
	sh.clusterCount = 0
}

func (sh *testshaper) SetStyle(styleChange styled.BidiStyleChange) {
}

func (sh *testshaper) ReadClusterMetrics() (clusterId int, width dimen.DU, len int, err error) {
	if sh.source == nil {
		return 0, 0, 0, io.EOF
	}
	var r rune = '\ufffd' // invalid rune signal by RuneReader.ReadRune(…)
	for err == nil && r == '\ufffd' {
		r, len, err = sh.source.ReadRune()
		tracer().Debugf("read rune = %q", r)
		if len == 0 {
			r = '\ufffd' // skip and continue
		}
	}
	if err != nil {
		return 0, 0, 0, err
	}
	clusterId = sh.clusterCount
	sh.clusterCount += 1
	width = 5 * dimen.PT
	tracer().Debugf("cluster=%d, width=%d, len=%d, err=%v", clusterId, width, len, err)
	return
}

// --- Khipukamayuq ----------------------------------------------------------

func newTestKq() *Khipukamayuq {
	env := typEnv{
		shaper: &testshaper{},
		params: newParameters(),
	}
	return NewKhipukamayuq(env)
}
