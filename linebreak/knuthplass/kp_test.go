package knuthplass

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/schuko/tracing/gotestingadapter"
	"github.com/npillmayer/uax/bidi"
	"golang.org/x/text/language"
)

var graphviz = false // globally switches GraphViz output on/off

func TestGraph1(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "tyse.frame")
	defer teardown()
	//
	parshape := linebreak.RectangularParShape(10 * 10 * dimen.BP)
	g := newLinebreaker(parshape, nil)
	g.newBreakpointAtMark(provisionalMark(1))
	if g.Breakpoint(1) == nil {
		t.Errorf("Expected to find breakpoint at %d in graph, is nil", 1)
	}
}

func setupKPTest(t *testing.T, paragraph string, hyphens bool) (*khipu.Khipu, linebreak.Cursor, io.Writer) {
	regs := newParameters()
	if hyphens {
		regs.Minhyphenlength = 3
	} else {
		regs.Minhyphenlength = 100
	}
	kh := khipu.KnotEncode(strings.NewReader(paragraph), 0, nil, regs)
	if kh == nil {
		t.Errorf("no Khipu to test; input is %s", paragraph)
	}
	kh.AppendKnot(khipu.Penalty(linebreak.InfinityMerits))
	cursor := linebreak.NewFixedWidthCursor(khipu.NewCursor(kh), 10*dimen.BP, 0)
	var dotfile io.Writer
	var err error
	if graphviz {
		dotfile, err = os.CreateTemp(".", "knuthplass-*.dot")
		if err != nil {
			t.Error(err)
		}
	}
	return kh, cursor, dotfile
}

func TestKPUnderfull(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "tyse.frame")
	defer teardown()
	//
	kh, cursor, dotfile := setupKPTest(t, " ", false)
	parshape := linebreak.RectangularParShape(10 * 10 * dimen.BP)
	v, breaks, err := FindBreakpoints(cursor, parshape, nil, dotfile)
	t.Logf("%d linebreaking-variants for empty line found, error = %v", len(v), err)
	for linecnt, breakpoints := range breaks {
		t.Logf("# Paragraph with %d lines: %v", linecnt, breakpoints)
		j := int64(0)
		for i := 1; i < len(v); i++ {
			t.Logf(": %s", kh.Text(j, breakpoints[i].Position()))
			j = breakpoints[i].Position()
		}
	}
	if err != nil || len(v) != 1 || len(breaks[1]) != 2 {
		t.Fail()
	}
}

func TestKPExactFit(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "tyse.frame")
	defer teardown()
	//
	kh, cursor, dotfile := setupKPTest(t, "The quick.", false)
	parshape := linebreak.RectangularParShape(10 * 10 * dimen.BP)
	v, breaks, err := FindBreakpoints(cursor, parshape, nil, dotfile)
	t.Logf("%d linebreaking-variants found, error = %v", len(v), err)
	for linecnt, breakpoints := range breaks {
		t.Logf("# Paragraph with %d lines: %v", linecnt, breakpoints)
		j := int64(0)
		for i := 1; i < len(v); i++ {
			t.Logf(": %s", kh.Text(j, breakpoints[i].Position()))
			j = breakpoints[i].Position()
		}
	}
	if err != nil || len(v) != 1 || len(breaks[1]) != 2 {
		t.Fail()
	}
}

func TestKPOverfull(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "tyse.frame")
	defer teardown()
	//
	kh, cursor, dotfile := setupKPTest(t, "The quick brown fox.", false)
	params := NewKPDefaultParameters()
	params.EmergencyStretch = dimen.DU(0)
	params.Tolerance = 400
	parshape := linebreak.RectangularParShape(10 * 10 * dimen.BP)
	v, breaks, err := FindBreakpoints(cursor, parshape, params, dotfile)
	t.Logf("%d linebreaking-variants found, error = %v", len(v), err)
	for linecnt, breakpoints := range breaks {
		t.Logf("# Paragraph with %d lines: %v", linecnt, breakpoints)
		j := int64(0)
		for i := 1; i < len(v); i++ {
			t.Logf(": %s", kh.Text(j, breakpoints[i].Position()))
			j = breakpoints[i].Position()
		}
	}
	if err != nil || len(v) != 1 || len(breaks[2]) != 3 {
		t.Fail()
	}
}

var princess = `In olden times when wishing still helped one, there lived a king whose daughters were all beautiful; and the youngest was so beautiful that the sun itself, which has seen so much, was astonished whenever it shone in her face. Close by the king's castle lay a great dark forest, and under an old lime-tree in the forest was a well, and when the day was very warm, the king's child went out into the forest and sat down by the side of the cool fountain; and when she was bored she took a golden ball, and threw it up on high and caught it; and this ball was her favorite plaything.`
var king = `In olden times when wishing still helped one, there lived a king`

func TestKPParaKing(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "tyse.frame")
	defer teardown()
	//
	kh, _, dotfile := setupKPTest(t, king, false)
	cursor := linebreak.NewFixedWidthCursor(khipu.NewCursor(kh), 10*dimen.BP, 3)
	params := NewKPDefaultParameters()
	parshape := linebreak.RectangularParShape(45 * 10 * dimen.BP)
	v, breaks, err := FindBreakpoints(cursor, parshape, params, dotfile)
	t.Logf("%d linebreaking-variants found, error = %v", len(v), err)
	for linecnt, breakpoints := range breaks {
		t.Logf("# Paragraph with %d lines: %v", linecnt, breakpoints)
		j := int64(0)
		for i := 1; i < len(v); i++ {
			t.Logf(": %s", kh.Text(j, breakpoints[i].Position()))
			j = breakpoints[i].Position()
		}
	}
	if err != nil || len(v) != 1 || len(breaks[2]) != 3 {
		t.Fail()
	}
}

func TestKPParaPrincess(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "tyse.frame")
	defer teardown()
	//
	kh, _, _ := setupKPTest(t, princess, false)
	// change to cursor with flexible interword-spacing
	cursor := linebreak.NewFixedWidthCursor(khipu.NewCursor(kh), 10*dimen.BP, 2)
	params := NewKPDefaultParameters()
	parshape := linebreak.RectangularParShape(45 * 10 * dimen.BP)
	breakpoints, err := BreakParagraph(cursor, parshape, params)
	//v, breaks, err := FindBreakpoints(cursor, parshape, params, dotfile)
	//t.Logf("%d linebreaking-variants found, error = %v", len(v), err)
	t.Logf("# Paragraph with %d lines: %v", len(breakpoints)-1, breakpoints)
	t.Logf("    |---------+---------+---------+---------+-----|")
	j := int64(0)
	for i := 1; i < len(breakpoints); i++ {
		//	t.Logf("%3d: %s", i, kh.Text(j, breakpoints[i].Position()))
		text := kh.Text(j, breakpoints[i].Position())
		t.Logf("%3d: %-45s|", i, justify(text, 45, i%2 == 0))
		j = breakpoints[i].Position()
	}
	if err != nil {
		t.Error(err)
	}
}

// crude implementation just for testing
func justify(text string, l int, even bool) string {
	t := strings.Trim(text, " \t\n")
	d := l - len(t)
	if d == 0 {
		return t // fit
	} else if d < 0 { // overfull box
		return text + "\u25ae"
	}
	s := strings.Fields(text)
	if len(s) == 1 {
		return text
	}
	var b bytes.Buffer
	W := 0 // length of all words
	for _, w := range s {
		W += len(w)
	}
	d = l - W // amount of WS to distribute
	ws := d / (len(s) - 1)
	r := d - ws*(len(s)-1) + 1
	b.WriteString(s[0])
	if even {
		for j := 1; j < r; j++ {
			for range ws + 1 {
				b.WriteString(" ")
			}
			b.WriteString(s[j])
		}
		for j := r; j < len(s); j++ {
			for range ws {
				b.WriteString(" ")
			}
			b.WriteString(s[j])
		}
	} else {
		for j := 1; j <= len(s)-r; j++ {
			for range ws {
				b.WriteString(" ")
			}
			b.WriteString(s[j])
		}
		for j := len(s) - r + 1; j < len(s); j++ {
			for range ws + 1 {
				b.WriteString(" ")
			}
			b.WriteString(s[j])
		}
	}
	return b.String()
}

func newParameters() *khipu.Params {
	var params khipu.Params
	params.Language = language.English
	params.Script = language.MustParseScript("Latn")
	params.BidiDir = bidi.LeftToRight
	params.Baselineskip = 12 * dimen.PT
	params.Lineskip = 0
	params.Lineskiplimit = 0
	params.Hypenchar = rune('-')
	params.Hyphenpenalty = 10
	params.Minhyphenlength = 2
	return &params
}
