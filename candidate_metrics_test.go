package khipu

import (
	"errors"
	"io"
	"math"
	"strings"
	"testing"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
)

func TestCandidateSourceClassification(t *testing.T) {
	for _, tc := range []struct {
		s    string
		want candidateSourceClass
	}{
		{"", candidateEmpty}, {"  ", candidateSpaces}, {"\u00a0", candidateOtherWhitespace},
		{"\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u202f\u205f\u3000", candidateOtherWhitespace},
		{"\t\r\n", candidateOtherWhitespace}, {" \u2003", candidateMixedWhitespace},
		{"alpha beta", candidateText}, {"\u200b", candidateText}, {"אב", candidateText},
	} {
		got, err := classifyCandidateSource(strings.NewReader(tc.s))
		if err != nil || got != tc.want {
			t.Fatalf("%q: %v %v", tc.s, got, err)
		}
		kind, flags, emit := got.kindAndFlags()
		if emit != (got != candidateEmpty) {
			t.Fatal("empty emission")
		}
		if emit && kind != kindFromText(tc.s) {
			t.Fatal("classification regression")
		}
		if emit && flags != defaultFlagsForKind(kind) {
			t.Fatal("flag regression")
		}
	}
	failure := errors.New("source failed")
	for _, r := range []io.Reader{nil, strings.NewReader("a\xff"), io.MultiReader(strings.NewReader(" "), boundaryErrorReader{failure})} {
		if _, err := classifyCandidateSource(r); err == nil {
			t.Fatal("accepted invalid reader")
		}
	}
}

type candidateScale func(int32) float64

func (f candidateScale) ToDU(v int32) float64 { return f(v) }

func TestCandidateWidthArithmetic(t *testing.T) {
	var w candidateWidth
	scale := candidateScale(func(v int32) float64 { return float64(v) / 4 })
	// Two fractional advances and a negative adjustment, not per-glyph rounding.
	if err := w.add([]contracts.ShapedGlyph{{XAdvance: 3, XOffset: 900}, {XAdvance: 4, YAdvance: 100}, {XAdvance: -1}}, scale); err != nil {
		t.Fatal(err)
	}
	got, err := w.finish(candidateSpaces, contracts.SpacingElastic, *newParameters())
	if err != nil || got != (candidateWidths{2, 1, 2}) {
		t.Fatalf("%+v %v", got, err)
	}
	// 1000 DU => 50% additional stretch and 33.3% shrink, not exact one-third.
	w.natural = 1000
	got, err = w.finish(candidateSpaces, contracts.SpacingElastic, *newParameters())
	if err != nil || got != (candidateWidths{1000, 667, 1500}) {
		t.Fatalf("%+v %v", got, err)
	}
	for _, class := range []candidateSourceClass{candidateOtherWhitespace, candidateMixedWhitespace, candidateText} {
		got, err = w.finish(class, contracts.SpacingElastic, *newParameters())
		if err != nil || got != (candidateWidths{1000, 1000, 1000}) {
			t.Fatalf("%+v %v", got, err)
		}
	}
	for _, tc := range []struct {
		spacing contracts.SpacingMode
		p       Params
	}{
		{contracts.SpacingFixed, *newParameters()}, {contracts.SpacingElastic, Params{}},
	} {
		got, err = w.finish(candidateSpaces, tc.spacing, tc.p)
		if err != nil || got != (candidateWidths{1000, 1000, 1000}) {
			t.Fatalf("%+v %v", got, err)
		}
	}
	p := Params{SpaceShrinkPercent: 100}
	got, err = w.finish(candidateSpaces, contracts.SpacingElastic, p)
	if err != nil || got.MinW != 0 {
		t.Fatal(got, err)
	}
}

func TestCandidateWidthErrors(t *testing.T) {
	for _, p := range []Params{
		{SpaceStretchPercent: -1}, {SpaceShrinkPercent: -1}, {SpaceShrinkPercent: 101},
		{SpaceStretchPercent: math.NaN()}, {SpaceShrinkPercent: math.Inf(1)},
		{SpaceStretchPercent: math.MaxFloat64},
	} {
		if _, err := (candidateWidth{1000}).finish(candidateSpaces, contracts.SpacingElastic, p); !errors.Is(err, contracts.ErrIllegalArguments) {
			t.Fatal(p, err)
		}
	}
	for _, v := range []float64{-1, math.NaN(), math.Inf(1), float64(dimen.Fil), float64(dimen.Fil) - 0.4} {
		if _, err := (candidateWidth{v}).finish(candidateText, contracts.SpacingFixed, Params{}); err == nil {
			t.Fatal("accepted", v)
		}
	}
	w := candidateWidth{2}
	if err := w.add([]contracts.ShapedGlyph{{XAdvance: 1}}, candidateScale(func(int32) float64 { return math.NaN() })); err == nil || w.natural != 2 {
		t.Fatal("invalid conversion committed")
	}
	if err := w.add(nil, nil); err == nil {
		t.Fatal("accepted nil scale")
	}
}

func TestSpaceParameterSnapshot(t *testing.T) {
	owner := NewKhipukamayuq(Config{})
	if owner.env.params.SpaceStretchPercent != 50 || owner.env.params.SpaceShrinkPercent != 33.3 {
		t.Fatal(owner.env.params)
	}
	p := Params{SpaceStretchPercent: 25, SpaceShrinkPercent: 10}
	owner = NewKhipukamayuq(Config{Params: &p})
	p.SpaceStretchPercent = 90
	if owner.env.params.SpaceStretchPercent != 25 {
		t.Fatal("parameters not copied")
	}
	owner = NewKhipukamayuq(Config{Params: &Params{}})
	if owner.env.params.SpaceStretchPercent != 0 || owner.env.params.SpaceShrinkPercent != 0 {
		t.Fatal("explicit zero overwritten")
	}
}
