package firstfit

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/schuko/tracing"
	"github.com/npillmayer/schuko/tracing/gotestingadapter"
	"github.com/npillmayer/uax/bidi"
	"golang.org/x/text/language"
)

var graphviz = false // global switch for GraphViz DOT output

func TestBuffer(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "tyse.frame")
	defer teardown()
	//
	lb, _ := newTestLinebreaker(t, "Hello World!", 20)
	k, ok := lb.peek()
	if !ok || k.Type() != khipu.KTTextBox {
		//t.Logf("lb.pos=%d, lb.mark=%d", lb.pos, lb.check)
		t.Logf("lb.pos=%d", lb.pos)
		t.Errorf("expected the first knot to be TextBox('Hello'), is %v", k)
	}
	if knot := lb.next(); k != knot {
		t.Errorf("first knot is %v, re-read knot is %v", k, knot)
	}
}

func TestBacktrack(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "tyse.frame")
	defer teardown()
	//
	lb, _ := newTestLinebreaker(t, "the quick brown fox jumps over the lazy dog.", 30)
	k := lb.next()
	lb.checkpoint()
	lb.next()
	lb.next()
	lb.next()
	knot := lb.backtrack()
	if k != knot {
		t.Errorf("remembered start knot is %v, backtracked knot is %v", k, knot)
	}
	k = lb.next()
	lb.checkpoint()
	knot = lb.backtrack()
	if k != knot {
		t.Errorf("remembered knot is %v, backtracked knot is %v", k, knot)
	}
}
func TestLinebreak(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "tyse.frame")
	defer teardown()
	//
	lb, kh := newTestLinebreaker(t, "the quick brown fox jumps over the lazy dog.", 30)
	breakpoints, err := lb.FindBreakpoints()
	if err != nil {
		t.Error(err)
	}
	if len(breakpoints)-1 != 2 {
		t.Errorf("expected 'princess' to occupy 2 lines, got %d", len(breakpoints)-1)
	}
	t.Logf("# Paragraph with %d lines: %v", len(breakpoints)-1, breakpoints)
	t.Logf("    |---------+---------+---------+|")
	j := int64(0)
	for i := 1; i < len(breakpoints); i++ {
		//	t.Logf("%3d: %s", i, kh.Text(j, breakpoints[i].Position()))
		text := kh.Text(j, breakpoints[i].Position())
		t.Logf("%3d: %-30s|", i, justify(text, 30, i%2 == 0))
		j = breakpoints[i].Position()
	}
}

var princess = `In olden times when wishing still helped one, there lived a king whose daughters were all beautiful; and the youngest was so beautiful that the sun itself, which has seen so much, was astonished whenever it shone in her face. Close by the king's castle lay a great dark forest, and under an old lime-tree in the forest was a well, and when the day was very warm, the king's child went out into the forest and sat down by the side of the cool fountain; and when she was bored she took a golden ball, and threw it up on high and caught it; and this ball was her favorite plaything.`

func TestPrincess(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "tyse.frame")
	defer teardown()
	//
	lb, kh := newTestLinebreaker(t, princess, 45)
	kh.AppendKnot(khipu.PenaltyItem(-10000)) // TODO: add parfillskip
	breakpoints, err := lb.FindBreakpoints()
	if err != nil {
		t.Error(err)
	}
	if len(breakpoints)-1 != 14 {
		t.Errorf("expected 'princess' to occupy 14 lines, got %d", len(breakpoints)-1)
	}
	t.Logf("# Paragraph with %d lines: %v", len(breakpoints)-1, breakpoints)
	t.Logf("     |---------+---------+---------+---------+-----|")
	j := int64(0)
	for i := 1; i < len(breakpoints); i++ {
		//	t.Logf("%3d: %s", i, kh.Text(j, breakpoints[i].Position()))
		text := kh.Text(j, breakpoints[i].Position())
		t.Logf("%3d: %-45s|", i, justify(text, 45, i%2 == 0))
		j = breakpoints[i].Position()
	}
}

// --- Helpers ----------------------------------------------------------

func newTestLinebreaker(t *testing.T, text string, len int) (*linebreaker, *khipu.KhipuAOS) {
	kh, cursor, _ := setupFFTest(t, text, false)
	parshape := linebreak.RectangularParShape(dimen.DU(len) * 10 * dimen.BP)
	lb, err := newLinebreaker(cursor, parshape, nil)
	if err != nil {
		t.Error(err)
	}
	return lb, kh
}

func setupFFTest(t *testing.T, paragraph string, hyphens bool) (*khipu.KhipuAOS, linebreak.Cursor, io.Writer) {
	tracing.Select("tyse.frame").SetTraceLevel(tracing.LevelError)
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
	//kh.AppendKnot(khipu.Penalty(linebreak.InfinityMerits))
	cursor := linebreak.NewFixedWidthCursor(khipu.NewCursor(kh), 10*dimen.BP, 0)
	var dotfile io.Writer
	var err error
	if graphviz {
		dotfile, err = os.CreateTemp(".", "firstfit-*.dot")
		if err != nil {
			t.Error(err)
		}
	}
	tracing.Select("tyse.frame").SetTraceLevel(tracing.LevelDebug)
	return kh, cursor, dotfile
}

// -------------------------------------------------------------

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
