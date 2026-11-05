package khipu

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"unicode"
	"unicode/utf8"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
)

// candidateSourceClass separates knot kind from elasticity. In particular,
// NBSP and typographic spaces remain glue under the existing classification
// policy but do not acquire U+0020 elasticity. No word detection happens here.
type candidateSourceClass uint8

const (
	candidateEmpty           candidateSourceClass = iota
	candidateSpaces                               // exclusively U+0020
	candidateOtherWhitespace                      // exclusively non-U+0020 Unicode whitespace
	candidateMixedWhitespace                      // requires separation where cluster boundaries allow
	candidateText
)

// classifyCandidateSource reads a bounded original source range without copying
// text. Mixed space kinds must not inherit elasticity for their entire width;
// the accumulator can split at cluster boundaries, or retain fixed width if an
// inseparable cluster contains mixed material. Empty ranges create no knot.
func classifyCandidateSource(source io.Reader) (candidateSourceClass, error) {
	if nilRunService(source) {
		return candidateEmpty, fmt.Errorf("%w: missing candidate source", contracts.ErrIllegalArguments)
	}
	r := bufio.NewReader(source)
	space, other, text := false, false, false
	for {
		c, n, err := r.ReadRune()
		if err == io.EOF {
			break
		}
		if err != nil {
			return candidateEmpty, err
		}
		if c == utf8.RuneError && n == 1 {
			return candidateEmpty, fmt.Errorf("%w: invalid candidate UTF-8", contracts.ErrIllegalArguments)
		}
		switch {
		case c == ' ':
			space = true
		case unicode.IsSpace(c):
			other = true
		default:
			text = true
		}
	}
	switch {
	case text:
		return candidateText, nil
	case space && other:
		return candidateMixedWhitespace, nil
	case space:
		return candidateSpaces, nil
	case other:
		return candidateOtherWhitespace, nil
	default:
		return candidateEmpty, nil
	}
}

// kindAndFlags preserves the current whitespace/discardability policy. A fixed
// width does not imply non-discardability. The bool is false for empty source.
func (c candidateSourceClass) kindAndFlags() (KnotType, KnotFlags, bool) {
	if c == candidateEmpty {
		return KTTextBox, 0, false
	}
	kind := KTTextBox
	if c != candidateText {
		kind = KTGlue
	}
	return kind, defaultFlagsForKind(kind), true
}

// candidateWidth accumulates horizontal advances in fractional DU. Offsets
// position ink and never contribute to pen advance; Y metrics are retained by
// glyph storage but are not used for horizontal line width. Negative advances
// are allowed, but the final natural width must be nonnegative.
type candidateWidth struct{ natural float64 }

func (w *candidateWidth) add(glyphs []contracts.ShapedGlyph, scale contracts.FontScale) error {
	if nilRunService(scale) {
		return fmt.Errorf("%w: missing font scale", contracts.ErrIllegalArguments)
	}
	sum := w.natural
	for _, g := range glyphs {
		v := scale.ToDU(g.XAdvance)
		if math.IsNaN(v) || math.IsInf(v, 0) || math.IsNaN(sum+v) || math.IsInf(sum+v, 0) {
			return fmt.Errorf("%w: nonfinite glyph width", contracts.ErrIllegalArguments)
		}
		sum += v
	}
	w.natural = sum
	return nil
}

type candidateWidths struct{ W, MinW, MaxW dimen.DU }

// finish rounds once per completed candidate, with ties away from zero. Space
// bounds are calculated from unrounded natural width, not its rounded integer.
// Reserve the dimension package's infinity/fill sentinels for explicit glue.
func (w candidateWidth) finish(class candidateSourceClass, spacing contracts.SpacingMode, params Params) (candidateWidths, error) {
	bad := func() (candidateWidths, error) {
		return candidateWidths{}, fmt.Errorf("%w: invalid candidate width or space elasticity", contracts.ErrIllegalArguments)
	}
	s, sh := params.SpaceStretchPercent, params.SpaceShrinkPercent
	if math.IsNaN(s) || math.IsInf(s, 0) || s < 0 || math.IsNaN(sh) || math.IsInf(sh, 0) || sh < 0 || sh > 100 ||
		(spacing != contracts.SpacingFixed && spacing != contracts.SpacingElastic) || class > candidateText ||
		math.IsNaN(w.natural) || math.IsInf(w.natural, 0) || w.natural < 0 {
		return bad()
	}
	minimum, maximum := w.natural, w.natural
	if class == candidateSpaces && spacing == contracts.SpacingElastic {
		minimum = w.natural * (1 - sh/100)
		maximum = w.natural * (1 + s/100)
	}
	values := []float64{w.natural, minimum, maximum}
	var rounded [3]dimen.DU
	for i, v := range values {
		v = math.Round(v)
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v >= float64(dimen.Fil) {
			return bad()
		}
		rounded[i] = dimen.DU(v)
	}
	return candidateWidths{rounded[0], rounded[1], rounded[2]}, nil
}
