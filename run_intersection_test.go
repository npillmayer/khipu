package khipu

import (
	"bufio"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/uax/bidi"
)

type intersectionStyle string

func (s intersectionStyle) Equals(other styled.Style) bool { return s == other }
func (s intersectionStyle) String() string                 { return string(s) }

func TestIntersectParagraphRuns(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		ranges     []runSourceRange
		dirs       []bidi.Direction
	}{
		{"empty", "", nil, nil},
		{"empty LTR isolate", "\u2066\u2069", nil, nil},
		{"empty RTL isolate", "\u2067\u2069", nil, nil},
		{"nested empty isolates", "\u2067\u2066\u2069\u2069", nil, nil},
		{"empty isolate between text", "a\u2066\u2069b", []runSourceRange{{0, 1, 0, 1}, {7, 8, 3, 4}}, []bidi.Direction{bidi.LeftToRight, bidi.LeftToRight}},
		{"leading empty isolate", "\u2066\u2069a", []runSourceRange{{6, 7, 2, 3}}, []bidi.Direction{bidi.LeftToRight}},
		{"trailing empty isolate", "a\u2066\u2069", []runSourceRange{{0, 1, 0, 1}}, []bidi.Direction{bidi.LeftToRight}},
		{"combining", "a\u0301é", []runSourceRange{{0, 5, 0, 3}}, []bidi.Direction{bidi.LeftToRight}},
		{"mixed", "Go אבג!", []runSourceRange{{0, 3, 0, 3}, {3, 9, 3, 6}, {9, 10, 6, 7}}, []bidi.Direction{bidi.LeftToRight, bidi.RightToLeft, bidi.LeftToRight}},
		{"nested", "a\u2067אב\u2066cd\u2069גד\u2069z", []runSourceRange{{0, 1, 0, 1}, {4, 8, 2, 4}, {11, 13, 5, 7}, {16, 20, 8, 10}, {23, 24, 11, 12}}, []bidi.Direction{bidi.LeftToRight, bidi.RightToLeft, bidi.LeftToRight, bidi.RightToLeft, bidi.LeftToRight}},
		{"same direction isolate", "a\u2066b\u2069c", []runSourceRange{{0, 1, 0, 1}, {4, 5, 2, 3}, {8, 9, 4, 5}}, []bidi.Direction{bidi.LeftToRight, bidi.LeftToRight, bidi.LeftToRight}},
		{"edge gaps", "\u2067אב\u2069", []runSourceRange{{3, 7, 1, 3}}, []bidi.Direction{bidi.RightToLeft}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text := styled.TextFromString(tc.text)
			para, err := styled.ParagraphFromText(&text, 0, uint64(len(tc.text)), bidi.LeftToRight, nil)
			if err != nil {
				t.Fatal(err)
			}
			before := slices.Collect(para.BidiLevels().LogicalSegments())
			got, err := intersectParagraphRuns(para)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(tc.ranges) {
				t.Fatalf("got %v", got)
			}
			for i, item := range got {
				if item.source != tc.ranges[i] || item.direction != tc.dirs[i] {
					t.Errorf("item %d: %+v", i, item)
				}
			}
			if after := slices.Collect(para.BidiLevels().LogicalSegments()); !reflect.DeepEqual(before, after) {
				t.Fatal("bidi resolution changed")
			}
			again, err := intersectParagraphRuns(para)
			if err != nil || !reflect.DeepEqual(got, again) {
				t.Fatal("repeat traversal differs", err)
			}
		})
	}
}

func TestIntersectParagraphStyles(t *testing.T) {
	text := styled.TextFromString("aאבz")
	text, err := text.Style(intersectionStyle("bold"), 1, 3)
	if err != nil {
		t.Fatal(err)
	}
	para, err := styled.ParagraphFromText(&text, 0, 6, bidi.LeftToRight, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := intersectParagraphRuns(para)
	if err != nil {
		t.Fatal(err)
	}
	want := []runSourceRange{{0, 1, 0, 1}, {1, 3, 1, 2}, {3, 5, 2, 3}, {5, 6, 3, 4}}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i].source != want[i] {
			t.Errorf("range %d: %+v", i, got[i])
		}
	}
	if got[1].style != intersectionStyle("bold") {
		t.Fatal("lost style association")
	}
}

func TestSourceScalarScannerRejectsInvalidInput(t *testing.T) {
	for _, tc := range []struct {
		text string
		end  uint64
	}{{"é", 1}, {"\xff", 1}, {"a", 2}} {
		s := sourceScalarScanner{reader: bufio.NewReader(strings.NewReader(tc.text))}
		if _, err := s.advance(tc.end); err == nil {
			t.Errorf("accepted %q endpoint %d", tc.text, tc.end)
		}
	}
	if got, err := intersectParagraphRuns(nil); err == nil || got != nil {
		t.Fatal("accepted nil paragraph")
	}
}
