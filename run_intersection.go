package khipu

import (
	"bufio"
	"fmt"
	"io"
	"iter"
	"unicode/utf8"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/uax/bidi"
)

// runIntersection associates original source coordinates with a style and a
// resolved direction. It owns no text. Font resolution and shaping are later
// stages; callers can read its text through Paragraph.Raw().BoundedReader.
type runIntersection struct {
	source    runSourceRange
	style     styled.Style
	direction bidi.Direction
}

// intersectParagraphRuns intersects style spans with logical bidi segments.
// The paragraph must be intact (not wrapped, split or reordered) and must not
// be modified during this call. Neither text nor bidi backing data is changed.
// Isolate boundaries remain distinct even when styles and directions agree.
// Omitted controls leave source gaps, but still count toward scalar positions.
// Errors return no partial result. This stage does not decide grapheme safety.
func intersectParagraphRuns(para *styled.Paragraph) ([]runIntersection, error) {
	if para == nil || para.BidiLevels() == nil {
		return nil, fmt.Errorf("khipu: intersection requires a resolved paragraph")
	}
	length := para.Raw().Len()
	scan := sourceScalarScanner{reader: bufio.NewReader(para.Reader())}
	next, stop := iter.Pull(para.BidiLevels().LogicalSegments())
	defer stop()
	var previousEnd uint64
	readSegment := func() (bidi.Segment, bool, error) {
		seg, ok := next()
		if ok {
			if seg.From < previousEnd || seg.From >= seg.To || seg.To > length ||
				(seg.Dir != bidi.LeftToRight && seg.Dir != bidi.RightToLeft) {
				return seg, false, fmt.Errorf("khipu: invalid logical bidi segment")
			}
			previousEnd = seg.To
		}
		return seg, ok, nil
	}
	seg, more, err := readSegment()
	if err != nil {
		return nil, err
	}
	var result []runIntersection
	var styleEnd uint64
	for span, _ := range para.StyleRanges() {
		if span.Position != styleEnd || span.Length == 0 || span.Position > length || span.Length > length-span.Position {
			return nil, fmt.Errorf("khipu: invalid style partition")
		}
		styleEnd = span.Position + span.Length
		if _, err := scan.advance(span.Position); err != nil {
			return nil, err
		}
		for more && seg.From < styleEnd {
			from, to := max(span.Position, seg.From), min(styleEnd, seg.To)
			start, err := scan.advance(from)
			if err != nil {
				return nil, err
			}
			end, err := scan.advance(to)
			if err != nil {
				return nil, err
			}
			result = append(result, runIntersection{
				source: runSourceRange{byteStart: from, byteEnd: to, scalarStart: start, scalarEnd: end},
				style:  span.Style, direction: seg.Dir,
			})
			if seg.To > styleEnd {
				break
			}
			seg, more, err = readSegment()
			if err != nil {
				return nil, err
			}
		}
		if _, err := scan.advance(styleEnd); err != nil {
			return nil, err
		}
	}
	if styleEnd != length || more {
		return nil, fmt.Errorf("khipu: incomplete style partition")
	}
	if _, err := scan.advance(length); err != nil {
		return nil, err
	}
	return result, nil
}

// sourceScalarScanner walks the original reader once, including omitted bidi
// controls. Only endpoint counts are retained, not a paragraph-sized byte map.
type sourceScalarScanner struct {
	reader         *bufio.Reader
	bytes, scalars uint64
}

func (s *sourceScalarScanner) advance(to uint64) (uint64, error) {
	if to < s.bytes {
		return 0, fmt.Errorf("khipu: source endpoints out of order")
	}
	for s.bytes < to {
		r, size, err := s.reader.ReadRune()
		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return 0, fmt.Errorf("khipu: reading paragraph: %w", err)
		}
		if r == utf8.RuneError && size == 1 {
			return 0, fmt.Errorf("khipu: invalid UTF-8 source")
		}
		s.bytes += uint64(size)
		s.scalars++
		if s.bytes > to {
			return 0, fmt.Errorf("khipu: endpoint inside UTF-8 encoding")
		}
	}
	return s.scalars, nil
}
