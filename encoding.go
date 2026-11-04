package khipu

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"unicode"
	"unicode/utf8"

	"github.com/npillmayer/cords/styled"
)

// EncodeParagraph shapes the original source bytes and returns a complete SOA
// linebreaking input in logical order. Pos/Len refer to the original paragraph,
// with Pos including para.Offset. Style boundaries do not authorize line breaks.
// A final non-discardable fill knot carries the paragraph's forced break.
// Errors return no partial paragraph. Text is not normalized here: normalization
// must happen before styled source ranges are established.
func (kq *Khipukamayuq) EncodeParagraph(para *styled.Paragraph, knotAdapt KnotStyler) (*Khipu, error) {
	if para == nil {
		return nil, ErrIllegalArguments
	}
	if kq == nil {
		return nil, ErrVoidKhipukamayuq
	}
	source := para.Raw().String()
	if !utf8.ValidString(source) {
		return nil, fmt.Errorf("khipu: paragraph is not valid UTF-8")
	}
	if uint64(len(source)) > math.MaxUint64-para.Offset {
		return nil, fmt.Errorf("khipu: source position overflow")
	}
	fill := kq.env.parFillSkip
	if fill.MinW > fill.W || fill.W > fill.MaxW {
		return nil, fmt.Errorf("khipu: invalid paragraph fill widths")
	}
	khp := kq.allocKhipu(0)
	khp.paragraph = para
	boundaries := paragraphBoundaries(source)
	// Empty paragraphs have no bidi/style runs to visit in cords.
	if len(source) > 0 {
		consumed := 0
		for style, reader := range para.BidiStyleRanges() {
			buf, err := io.ReadAll(reader)
			if err != nil {
				return unwireKhipu(khp), err
			}
			start := style.Position
			if start != consumed || style.Length != len(buf) || start < 0 || start > len(source)-len(buf) || !utf8.Valid(buf) || !bytes.Equal(buf, []byte(source[start:start+len(buf)])) {
				return unwireKhipu(khp), fmt.Errorf("khipu: inconsistent style/source range")
			}
			kq.env.shaper.SetSource(wrapAsRuneReader(nil, buf))
			kq.env.shaper.SetStyle(style)
			metrics, err := getMetrics(buf, kq.env.shaper)
			if err != nil {
				return unwireKhipu(khp), err
			}
			from := 0
			for to := 1; to <= len(buf); to++ {
				if !metrics.starts[to] {
					continue
				}
				if to < len(buf) && boundaries[start+to] >= MaxPenalty && !spaceBoundary(buf, to) {
					continue
				}
				if to-from > math.MaxUint16 {
					return unwireKhipu(khp), fmt.Errorf("khipu: knot source span exceeds %d bytes", math.MaxUint16)
				}
				kind := kindFromText(string(buf[from:to]))
				w, err := metrics.width(from, to)
				if err != nil {
					return unwireKhipu(khp), err
				}
				k := KnotCore{Pos: para.Offset + uint64(start+from), Len: uint16(to - from), W: w, MinW: w, MaxW: w,
					Kind: kind, Flags: defaultFlagsForKind(kind), Penalty: boundaries[start+to]}
				if knotAdapt != nil {
					mn, mx := knotAdapt.AdaptKnot(k, style.Style)
					k = sanitizeKnot(k, mn, mx)
				}
				khp.appendKnot(k)
				from = to
			}
			consumed += len(buf)
		}
		if consumed != len(source) {
			return unwireKhipu(khp), fmt.Errorf("khipu: incomplete style/source ranges")
		}
	}
	// EOF belongs to the trailer, not to a second forced break on source content.
	if n := len(khp.Kind); n > 0 {
		khp.Penalty[n-1] = MaxPenalty
	}
	fill.Pos = para.Offset + uint64(len(source))
	fill.Len = 0
	fill.Kind = KTGlue
	fill.Flags = 0
	fill.Penalty = MaxMerit
	khp.appendKnot(fill)
	return khp, nil
}

// Split whitespace from content for trimming even when that boundary prohibits
// line breaking. Only the shaper's actual cluster boundaries may be split.
func spaceBoundary(buf []byte, at int) bool {
	left, _ := utf8.DecodeLastRune(buf[:at])
	right, _ := utf8.DecodeRune(buf[at:])
	return unicode.IsSpace(left) != unicode.IsSpace(right)
}
