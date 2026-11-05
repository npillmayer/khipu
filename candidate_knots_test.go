package khipu

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
)

func candidateFixture(t *testing.T, text string) (*candidateKnotAccumulator, *glyphClusterAssembler) {
	t.Helper()
	p := preparationParagraph(t, text)
	d := preparationDefaults()
	d.Spacing = contracts.SpacingElastic
	runs, err := prepareParagraphRuns(p, helpers.DefaultStyleResolver{}, d)
	if err != nil {
		t.Fatal(err)
	}
	boundaries, err := prepareParagraphBoundaries(p)
	if err != nil {
		t.Fatal(err)
	}
	a, err := newCandidateKnotAccumulator(p, runs, boundaries, *newParameters(), KnotCore{MaxW: dimen.Fill})
	if err != nil {
		t.Fatal(err)
	}
	g, err := newGlyphClusterAssembler(runs, boundaries)
	if err != nil {
		t.Fatal(err)
	}
	return a, g
}

func TestCandidateRejectsUnsupportedGapBreaks(t *testing.T) {
	a, g := candidateFixture(t, "a\u2067אב\u2069z")
	a.boundaries[2].penalty = MaxMerit
	a.boundaries[3].penalty = MaxMerit
	_, _ = g.push(contracts.ShapedGlyph{Cluster: 0})
	c, _ := g.push(contracts.ShapedGlyph{Cluster: 5})
	if err := a.push(c); err != nil {
		t.Fatal(err)
	}
	c, _ = g.finish()
	if err := a.push(c); err == nil {
		t.Fatal("collapsed mandatory breaks")
	}
	if result, err := a.finish(); result != nil || err == nil {
		t.Fatal("published failed paragraph")
	}
}

func TestCandidateWidthFailureIsAtomic(t *testing.T) {
	a, g := candidateFixture(t, "a b")
	a.runs[0].params.scale = candidateScale(func(int32) float64 { return math.Inf(1) })
	_, _ = g.push(contracts.ShapedGlyph{Cluster: 0, XAdvance: 500})
	c, _ := g.finish()
	if err := a.push(c); err == nil {
		t.Fatal("accepted infinite width")
	}
	if result, err := a.finish(); result != nil || err == nil {
		t.Fatal("published failed width")
	}
}

func accumulateTestGlyphs(t *testing.T, a *candidateKnotAccumulator, g *glyphClusterAssembler, glyphs []contracts.ShapedGlyph) []candidateKnot {
	t.Helper()
	for _, glyph := range glyphs {
		c, err := g.push(glyph)
		if err != nil {
			t.Fatal(err)
		}
		if c != nil {
			if err := a.push(c); err != nil {
				t.Fatal(err)
			}
		}
	}
	c, err := g.finish()
	if err != nil {
		t.Fatal(err)
	}
	if c != nil {
		if err := a.push(c); err != nil {
			t.Fatal(err)
		}
	}
	knots, err := a.finish()
	if err != nil {
		t.Fatal(err)
	}
	return knots
}

func TestCandidateKnotsStreaming(t *testing.T) {
	a, g := candidateFixture(t, "alpha beta")
	s, _ := helpers.NewSyntheticStreamingShaper(1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	glyphs, err := collectProduced(ctx, a.para, a.runs, s)
	if err != nil {
		t.Fatal(err)
	}
	knots := accumulateTestGlyphs(t, a, g, glyphs)
	if len(knots) != 4 {
		t.Fatalf("got %d knots", len(knots))
	}
	if knots[0].core.Len != 5 || knots[0].core.Penalty != MaxPenalty {
		t.Fatal(knots[0].core)
	}
	space := knots[1].core
	if space.Kind != KTGlue || space.W != 5*dimen.PT || space.MinW >= space.W || space.MaxW <= space.W || space.Flags != KFDiscardable {
		t.Fatal(space)
	}
	if knots[2].core.Penalty != MaxPenalty {
		t.Fatal("final source break")
	}
	trailer := knots[3]
	if trailer.core.Flags != 0 || trailer.core.Penalty != MaxMerit || trailer.core.MaxW != dimen.Fill || trailer.core.Pos != 10 || len(trailer.clusters) != 0 {
		t.Fatal(trailer)
	}
}

func TestCandidateDoesNotSplitCluster(t *testing.T) {
	a, g := candidateFixture(t, "a b")
	knots := accumulateTestGlyphs(t, a, g, []contracts.ShapedGlyph{{Cluster: 0, XAdvance: 1500}, {Cluster: 0, XOffset: 20}})
	if len(knots) != 2 || len(knots[0].clusters[0].glyphs) != 2 || knots[0].core.Kind != KTTextBox || knots[0].core.MinW != knots[0].core.MaxW {
		t.Fatal(knots)
	}
}

func TestCandidateOmittedZeroWidthSpace(t *testing.T) {
	a, g := candidateFixture(t, "a\u200bb")
	// No glyph for scalar 1: its source boundary ends the first cluster.
	knots := accumulateTestGlyphs(t, a, g, []contracts.ShapedGlyph{{Cluster: 0, XAdvance: 500}, {Cluster: 2, XAdvance: 500}})
	if len(knots) != 3 || knots[0].core.Penalty >= MaxPenalty || knots[0].breakAfter.byteEnd != 4 {
		t.Fatal(knots)
	}
}

func TestCandidateSilentRunBoundary(t *testing.T) {
	a, g := candidateFixture(t, "a\u2067אב\u2069z")
	// Script an opportunity in a completely silent run, independently of
	// the installed Unicode rules. No fake glyph or zero-width knot is needed.
	a.boundaries[3].penalty = 0
	knots := accumulateTestGlyphs(t, a, g, []contracts.ShapedGlyph{{Cluster: 0, XAdvance: 500}, {Cluster: 5, XAdvance: 500}})
	if len(knots) != 3 || knots[0].breakAfter.scalarEnd != 4 || knots[0].core.Len != 1 || knots[0].core.Penalty != 0 {
		t.Fatal(knots)
	}
}

func TestCandidateSpaceKindsAndOffsets(t *testing.T) {
	a, g := candidateFixture(t, " \u00a0\u2003 ")
	a.para.Offset = 100
	var glyphs []contracts.ShapedGlyph
	for i := uint32(0); i < 4; i++ {
		glyphs = append(glyphs, contracts.ShapedGlyph{Cluster: i, XAdvance: 500})
	}
	knots := accumulateTestGlyphs(t, a, g, glyphs)
	for _, k := range knots[:len(knots)-1] {
		if k.source.scalarStart == 0 || k.source.scalarStart == 3 {
			if k.core.MaxW <= k.core.W {
				t.Fatal("space not elastic", k)
			}
		} else if k.core.MaxW != k.core.W || k.core.MinW != k.core.W {
			t.Fatal("non-SPACE elastic", k)
		}
		if k.core.Pos != 100+k.source.byteStart {
			t.Fatal("lost paragraph offset")
		}
	}
}

func TestCandidateEmptyAndFailure(t *testing.T) {
	for _, text := range []string{"", "abc", "\u2066\u2069"} {
		a, g := candidateFixture(t, text)
		knots := accumulateTestGlyphs(t, a, g, nil)
		if len(knots) != 1 || knots[0].core.Penalty != MaxMerit {
			t.Fatal(knots)
		}
	}
	a, g := candidateFixture(t, "ab")
	_, _ = g.push(contracts.ShapedGlyph{Cluster: 0, XAdvance: 500})
	c, _ := g.push(contracts.ShapedGlyph{Cluster: 1, XAdvance: 500})
	if err := a.push(c); err != nil {
		t.Fatal(err)
	}
	// Duplicate source ranges poison the accumulator and discard prior output.
	if err := a.push(c); !errors.Is(err, contracts.ErrIllegalArguments) {
		t.Fatal(err)
	}
	if result, err := a.finish(); result != nil || err == nil {
		t.Fatal("published partial output")
	}
}

func TestCandidateRunBoundaryAndOwnership(t *testing.T) {
	a, g := candidateFixture(t, "aאב")
	knots := accumulateTestGlyphs(t, a, g, []contracts.ShapedGlyph{{Cluster: 0, XAdvance: 500}, {Cluster: 1, XAdvance: 500}, {Cluster: 2, XAdvance: 500}})
	if len(knots) != 3 || knots[0].clusters[0].runIndex == knots[1].clusters[0].runIndex {
		t.Fatal(knots)
	}
	a, g = candidateFixture(t, "ab")
	_, _ = g.push(contracts.ShapedGlyph{Cluster: 0, XAdvance: 500})
	c, _ := g.finish()
	if err := a.push(c); err != nil {
		t.Fatal(err)
	}
	c.glyphs[0].GlyphID = 999
	knots, err := a.finish()
	if err != nil {
		t.Fatal(err)
	}
	if knots[0].clusters[0].glyphs[0].GlyphID == 999 {
		t.Fatal("borrowed glyph storage")
	}
}
