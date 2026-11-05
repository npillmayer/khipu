package khipu

import (
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/npillmayer/khipu/contracts"
)

func TestReadParagraphBoundariesMatchesMetricPolicy(t *testing.T) {
	for _, source := range []string{
		"", "hello", "hello world ", "a\u00a0b", "a\r\nb", "a\nb", "a\u2028b",
		"aéz", "e\u0301", "\ufffd", "a\x00b", "a\u200bb", "a\u2060b",
		"a\u2067אב\u2069z", "\u2066\u2069", "中文测试", "(one) two!",
	} {
		t.Run(source, func(t *testing.T) {
			// One-byte reads exercise UTF-8 encodings split across reader calls.
			got, err := readParagraphBoundaries(iotest.OneByteReader(strings.NewReader(source)))
			if err != nil {
				t.Fatal(err)
			}
			legacy := paragraphBoundaries(source)
			var ends []uint64
			for at := range source {
				if at > 0 {
					ends = append(ends, uint64(at))
				}
			}
			if source != "" {
				ends = append(ends, uint64(len(source)))
			}
			if len(got) != len(ends) {
				t.Fatalf("got %v, want %d entries", got, len(ends))
			}
			for i, entry := range got {
				if entry.byteEnd != ends[i] || entry.scalarEnd != uint64(i+1) || entry.penalty != legacy[ends[i]] {
					t.Fatalf("entry %d: %+v, expected endpoint %d penalty %v", i, entry, ends[i], legacy[ends[i]])
				}
			}
		})
	}
}

func TestParagraphBoundariesIgnoreStyleAndRetainControls(t *testing.T) {
	p := preparationParagraph(t, "a\u2067אב\u2069z")
	want, err := prepareParagraphBoundaries(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Style(intersectionStyle("bold"), 4, 6); err != nil {
		t.Fatal(err)
	}
	got, err := prepareParagraphBoundaries(p)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("style changed boundaries: %v %v", got, err)
	}
	ends := []uint64{1, 4, 6, 8, 11, 12}
	if len(got) != len(ends) {
		t.Fatal(got)
	}
	for i, end := range ends {
		if got[i].byteEnd != end || got[i].scalarEnd != uint64(i+1) {
			t.Fatal(got)
		}
	}
}

type boundaryErrorReader struct{ err error }

func (r boundaryErrorReader) Read([]byte) (int, error) { return 0, r.err }

func TestBoundaryPreparationErrors(t *testing.T) {
	failure := errors.New("read failed")
	for _, tc := range []struct {
		name   string
		reader io.Reader
		want   error
	}{
		{"nil", nil, contracts.ErrIllegalArguments},
		{"malformed", strings.NewReader("valid\xff"), contracts.ErrIllegalArguments},
		{"truncated", strings.NewReader("a\xe2\x82"), contracts.ErrIllegalArguments},
		{"reader failure", io.MultiReader(strings.NewReader("valid"), boundaryErrorReader{failure}), failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := readParagraphBoundaries(tc.reader)
			if got != nil || !errors.Is(err, tc.want) {
				t.Fatalf("%v %v", got, err)
			}
		})
	}
	if got, err := prepareParagraphBoundaries(nil); got != nil || !errors.Is(err, contracts.ErrIllegalArguments) {
		t.Fatalf("%v %v", got, err)
	}
}

func TestBoundaryPreparationFinalReadWithEOF(t *testing.T) {
	source := "aé z"
	got, err := readParagraphBoundaries(iotest.DataErrReader(strings.NewReader(source)))
	if err != nil {
		t.Fatal(err)
	}
	want, err := readParagraphBoundaries(strings.NewReader(source))
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("%v %v", got, err)
	}
}

func TestWordBoundaryHints(t *testing.T) {
	for _, tc := range []struct {
		text string
		ends []uint64
	}{
		{"", nil},
		{"hello world", []uint64{5, 6, 11}},
		{"can't", []uint64{5}},
		{"3.14", []uint64{4}},
		{"a,b", []uint64{1, 2, 3}},
		{"a  b", []uint64{1, 3, 4}},
		{"e\u0301!", []uint64{3, 4}},
		{"a\r\nb", []uint64{1, 3, 4}},
		{"中文", []uint64{3, 6}},
		{"a\u2066\u2069b", []uint64{8}},
	} {
		t.Run(tc.text, func(t *testing.T) {
			got, err := readParagraphBoundaries(iotest.OneByteReader(strings.NewReader(tc.text)))
			if err != nil {
				t.Fatal(err)
			}
			var ends []uint64
			for _, b := range got {
				if b.wordBoundary {
					ends = append(ends, b.byteEnd)
				}
			}
			if !reflect.DeepEqual(ends, tc.ends) {
				t.Fatalf("word ends %v, want %v", ends, tc.ends)
			}
		})
	}
}

func TestWordHintDoesNotAuthorizeLineBreak(t *testing.T) {
	got, err := readParagraphBoundaries(strings.NewReader("a b"))
	if err != nil {
		t.Fatal(err)
	}
	if !got[0].wordBoundary || got[0].penalty != MaxPenalty {
		t.Fatalf("before space: expected word hint but prohibited line break, got %+v", got[0])
	}
}

func TestHardBreakPrecedence(t *testing.T) {
	for _, text := range []string{"\n\n", "\r\r", "\u2028\u2028", "\u0085\u0085"} {
		b, err := readParagraphBoundaries(strings.NewReader(text))
		if err != nil {
			t.Fatal(err)
		}
		if len(b) != 2 || b[0].penalty != MaxMerit || b[1].penalty != MaxMerit {
			t.Fatalf("%q: %+v", text, b)
		}
	}
	b, err := readParagraphBoundaries(strings.NewReader("\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	if b[0].penalty != MaxPenalty || b[1].penalty != MaxMerit {
		t.Fatal("CRLF must remain one separator", b)
	}
}
