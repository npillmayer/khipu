package khipu_test

import (
	"io"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/khipu/linebreak/firstfit"
	"github.com/npillmayer/khipu/linebreak/knuthplass"
	"github.com/npillmayer/uax/bidi"
)

func encodingParagraph(t *testing.T, source string) *styled.Paragraph {
	t.Helper()
	text := styled.TextFromString(source)
	para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), bidi.LeftToRight, nil)
	if err != nil {
		t.Fatal(err)
	}
	return para
}

func TestEncodedParagraphContract(t *testing.T) {
	owner := khipu.NewKhipukamayuq(khipu.Config{})
	for _, source := range []string{"breaking", "", "   ", "one two  ", "éclair café", "e\u0301lan", "a\ufffdb", "abc אבג xyz"} {
		t.Run(source, func(t *testing.T) {
			para := encodingParagraph(t, source)
			para.Offset = 100
			khp, err := owner.EncodeParagraph(para, nil)
			if err != nil {
				t.Fatal(err)
			}
			count := len(khp.Kind)
			if count == 0 {
				t.Fatal("missing trailer")
			}
			for _, n := range []int{len(khp.W), len(khp.MinW), len(khp.MaxW), len(khp.Penalty), len(khp.Pos), len(khp.Len), len(khp.Flags)} {
				if n != count {
					t.Fatalf("inconsistent SOA planes: %d != %d", n, count)
				}
			}
			end := khp.KnotByIndex(count - 1)
			if end.Kind != khipu.KTGlue || end.Len != 0 || end.Pos != 100+uint64(len(source)) || end.IsDiscardable() || end.Penalty != khipu.MaxMerit || end.W != 0 || end.MinW != 0 || end.MaxW != dimen.Fill {
				t.Fatalf("invalid paragraph trailer: %+v", end)
			}
			var reconstructed strings.Builder
			var total dimen.DU
			pos := uint64(100)
			for i := 0; i < count-1; i++ {
				knot := khp.KnotByIndex(i)
				if knot.Pos != pos || knot.Len == 0 {
					t.Fatalf("invalid source span at %d: %+v", i, knot)
				}
				start, stop := int(knot.Pos-100), int(knot.Pos-100)+int(knot.Len)
				if stop > len(source) || !utf8.ValidString(source[start:stop]) {
					t.Fatalf("invalid byte span: %+v", knot)
				}
				reconstructed.WriteString(source[start:stop])
				pos += uint64(knot.Len)
				total += knot.W
			}
			if reconstructed.String() != source {
				t.Fatalf("source changed: %q", reconstructed.String())
			}
			if total != dimen.DU(utf8.RuneCountInString(source))*5*dimen.PT {
				t.Fatalf("incorrect fixed-shaper width: %v", total)
			}
			breaks, err := firstfit.BreakParagraph(khp, linebreak.RectangularParShape(1000*dimen.PT), nil)
			if err != nil || !slices.Equal(breaks, []int{count - 1}) {
				t.Fatalf("firstfit: breaks=%v err=%v", breaks, err)
			}
			breaks, err = knuthplass.BreakParagraph(khp, linebreak.RectangularParShape(1000*dimen.PT), nil)
			if err != nil || !slices.Equal(breaks, []int{count - 1}) {
				t.Fatalf("Knuth-Plass: breaks=%v err=%v", breaks, err)
			}
		})
	}
}

type encodingStyle string

func (s encodingStyle) Equals(other styled.Style) bool { return s == other }
func (s encodingStyle) String() string                 { return string(s) }

func TestEncodedParagraphStyleAndForcedBoundaries(t *testing.T) {
	for _, source := range []string{"alphabet", "one\ntwo", "one\r\ntwo", "one\u2028two", "one\u00a0two", "one\ntwo\nthree\nfour"} {
		t.Run(source, func(t *testing.T) {
			para := encodingParagraph(t, source)
			if _, err := para.Style(encodingStyle("bold"), 1, 3); err != nil {
				t.Fatal(err)
			}
			owner := khipu.NewKhipukamayuq(khipu.Config{})
			khp, err := owner.EncodeParagraph(para, nil)
			if err != nil {
				t.Fatal(err)
			}
			forced := []int{}
			for i := range len(khp.Kind) {
				if khp.Penalty[i] == khipu.MaxMerit {
					forced = append(forced, i)
				}
			}
			wantLines := 1 + strings.Count(source, "\n") + strings.Count(source, "\u2028")
			if len(forced) != wantLines {
				t.Fatalf("forced boundaries=%v, want %d", forced, wantLines)
			}
			for _, run := range []struct {
				name      string
				breakPara func(*khipu.Khipu) ([]int, error)
			}{
				{"firstfit", func(k *khipu.Khipu) ([]int, error) {
					return firstfit.BreakParagraph(k, linebreak.RectangularParShape(1000*dimen.PT), nil)
				}},
				{"Knuth-Plass", func(k *khipu.Khipu) ([]int, error) {
					return knuthplass.BreakParagraph(k, linebreak.RectangularParShape(1000*dimen.PT), nil)
				}},
			} {
				breaks, err := run.breakPara(khp)
				if err != nil || !slices.Equal(breaks, forced) {
					t.Fatalf("%s: breaks=%v forced=%v err=%v", run.name, breaks, forced, err)
				}
			}
			if source == "alphabet" || strings.Contains(source, "\u00a0") {
				for i := 0; i < len(khp.Kind)-1; i++ {
					if khp.Penalty[i] != khipu.MaxPenalty {
						t.Fatalf("unexpected break at style/nonbreaking boundary: %d", i)
					}
				}
			}
		})
	}
}

type encodingHyphenator struct{ words []string }

func (h *encodingHyphenator) Discretionaries(word string, params *khipu.Params) ([]khipu.HyphenationFragment, error) {
	h.words = append(h.words, word)
	if word != "breaking" {
		return nil, nil
	}
	return []khipu.HyphenationFragment{{Variant: 7, PreBreak: "break-", PostBreak: "ing"}}, nil
}

func TestEncodedParagraphLazyDiscretionaries(t *testing.T) {
	for _, algorithm := range []string{"firstfit", "Knuth-Plass"} {
		t.Run(algorithm, func(t *testing.T) {
			hyph := &encodingHyphenator{}
			owner := khipu.NewKhipukamayuq(khipu.Config{Hyphenator: hyph})
			para := encodingParagraph(t, "xx breaking")
			para.Offset = 70
			if _, err := para.Style(encodingStyle("bold"), 3, 11); err != nil {
				t.Fatal(err)
			}
			khp, err := owner.EncodeParagraph(para, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(hyph.words) != 0 {
				t.Fatal("hyphenation must stay lazy during encoding")
			}
			shape := linebreak.RectangularParShape(45 * dimen.PT)
			var breaks []int
			if algorithm == "firstfit" {
				breaks, err = firstfit.BreakParagraph(khp, shape, &firstfit.Parameters{DiscretionaryProvider: owner})
			} else {
				params := knuthplass.NewKPDefaultParameters()
				params.RightSkip = khipu.KnotCore{MaxW: 5 * dimen.PT, Kind: khipu.KTGlue}
				params.DiscretionaryProvider = owner
				breaks, err = knuthplass.BreakParagraph(khp, shape, params)
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(breaks) != 2 || len(khp.SelectedDiscretionaries) != 1 {
				t.Fatalf("breaks=%v selections=%v", breaks, khp.SelectedDiscretionaries)
			}
			at := breaks[0]
			chosen, ok := khp.SelectedDiscretionaryAt(at)
			if !ok || chosen.Source != at || chosen.Variant != 7 || khp.Pos[at] != 73 {
				t.Fatalf("selection=%+v pos=%d", chosen, khp.Pos[at])
			}
			cands := khp.DiscretionariesAt(at)
			if len(cands) != 1 || cands[0].PreBreak.W != 30*dimen.PT || cands[0].PostBreak.W != 15*dimen.PT {
				t.Fatalf("candidate metrics=%+v", cands)
			}
			if !slices.Equal(hyph.words, []string{"breaking"}) {
				t.Fatalf("hyphenator source=%v", hyph.words)
			}
		})
	}
}

func TestEncodedParagraphFillConfiguration(t *testing.T) {
	fill := khipu.KnotCore{W: 2 * dimen.PT, MinW: 1 * dimen.PT, MaxW: 7 * dimen.PT, Kind: khipu.KTTextBox, Flags: khipu.KFDiscardable, Penalty: 10, Pos: 9, Len: 9}
	owner := khipu.NewKhipukamayuq(khipu.Config{ParFillSkip: &fill})
	fill.W = 100 * dimen.PT
	khp, err := owner.EncodeParagraph(encodingParagraph(t, "a "), nil)
	if err != nil {
		t.Fatal(err)
	}
	last := len(khp.Kind) - 1
	end := khp.KnotByIndex(last)
	if end.W != 2*dimen.PT || end.MinW != dimen.PT || end.MaxW != 7*dimen.PT || end.Kind != khipu.KTGlue || end.IsDiscardable() || end.Penalty != khipu.MaxMerit || end.Pos != 2 || end.Len != 0 {
		t.Fatalf("invalid configured trailer: %+v", end)
	}
	khp.Unskip()
	if len(khp.Kind) != last+1 {
		t.Fatal("Unskip removed non-discardable trailer")
	}
	for _, fill := range []khipu.KnotCore{{}, {W: 2, MinW: 3, MaxW: 4}, {W: 3, MaxW: 2}} {
		owner := khipu.NewKhipukamayuq(khipu.Config{ParFillSkip: &fill})
		khp, err := owner.EncodeParagraph(encodingParagraph(t, ""), nil)
		if fill == (khipu.KnotCore{}) {
			if err != nil || khp.MaxW[0] != 0 {
				t.Fatalf("explicit zero fill: %v %v", khp, err)
			}
		} else if err == nil || khp != nil {
			t.Fatalf("invalid fill accepted: %v %v", khp, err)
		}
	}
}

// Sparse metrics model a ligature spanning a potential break, or a zero-width
// space cluster. Both cases defeated the old width-as-cluster-marker approach.
type encodingClusterShaper struct {
	anchors []int
	widths  []dimen.DU
	next    int
}

func (s *encodingClusterShaper) SetSource(khipu.RuneReader)    { s.next = 0 }
func (*encodingClusterShaper) SetStyle(styled.BidiStyleChange) {}
func (s *encodingClusterShaper) ReadClusterMetrics() (int, dimen.DU, int, error) {
	if s.next == len(s.anchors) {
		return 999, 999, 0, io.EOF
	}
	i := s.next
	s.next++
	return s.anchors[i], s.widths[i], 0, nil
}

func TestEncodedParagraphClusterSpans(t *testing.T) {
	for _, tc := range []struct {
		name    string
		anchors []int
		widths  []dimen.DU
		spans   []uint16
	}{
		{"crosses space", []int{0, 4}, []dimen.DU{10, 5}, []uint16{5, 0}},
		{"zero width space", []int{0, 2, 3, 4}, []dimen.DU{10, 0, 5, 5}, []uint16{2, 1, 2, 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			shaper := &encodingClusterShaper{anchors: tc.anchors, widths: tc.widths}
			owner := khipu.NewKhipukamayuq(khipu.Config{Shaper: shaper})
			para := encodingParagraph(t, "ab cd")
			for range 2 {
				khp, err := owner.EncodeParagraph(para, nil)
				if err != nil {
					t.Fatal(err)
				}
				if !slices.Equal(khp.Len, tc.spans) {
					t.Fatalf("source spans=%v want %v", khp.Len, tc.spans)
				}
				if len(tc.spans) > 2 && khp.Penalty[1] >= khipu.MaxPenalty {
					t.Fatal("lost zero-width space break")
				}
			}
		})
	}
}

func TestEncodedParagraphSourceSection(t *testing.T) {
	text := styled.TextFromString("prefix éclair café suffix")
	from, to := uint64(len("prefix ")), uint64(len("prefix éclair café"))
	para, err := styled.ParagraphFromText(&text, from, to, bidi.LeftToRight, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := para.Style(encodingStyle("bold"), 0, uint64(len("éclair"))); err != nil {
		t.Fatal(err)
	}
	owner := khipu.NewKhipukamayuq(khipu.Config{})
	for range 2 {
		khp, err := owner.EncodeParagraph(para, nil)
		if err != nil {
			t.Fatal(err)
		}
		if khp.Pos[0] != from || khp.Pos[len(khp.Pos)-1] != to || khp.Len[0] != uint16(len("éclair")) || khp.W[0] != 30*dimen.PT {
			t.Fatalf("section positions/metrics: %+v", khp)
		}
	}
}

func TestEncodedParagraphErrorsReturnNoPartialOutput(t *testing.T) {
	para := encodingParagraph(t, "ab cd")
	shaper := &encodingClusterShaper{anchors: []int{0, -1}, widths: []dimen.DU{5, 5}}
	owner := khipu.NewKhipukamayuq(khipu.Config{Shaper: shaper})
	if khp, err := owner.EncodeParagraph(para, nil); err == nil || khp != nil {
		t.Fatalf("invalid shaper must return no output: %v %v", khp, err)
	}
	shaper.anchors = []int{0, 4}
	if _, err := owner.EncodeParagraph(para, nil); err != nil {
		t.Fatalf("owner cannot be reused after error: %v", err)
	}
	para.Offset = ^uint64(0)
	if khp, err := owner.EncodeParagraph(para, nil); err == nil || khp != nil {
		t.Fatalf("source offset overflow accepted: %v %v", khp, err)
	}
	// One unbreakable word cannot fit in the current uint16 source-length plane.
	para = encodingParagraph(t, strings.Repeat("a", 1<<16))
	owner = khipu.NewKhipukamayuq(khipu.Config{})
	if khp, err := owner.EncodeParagraph(para, nil); err == nil || khp != nil {
		t.Fatalf("oversized source span accepted: %v %v", khp, err)
	}
}
