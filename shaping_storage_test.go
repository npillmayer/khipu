package khipu

import (
	"context"
	"testing"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
)

func storageCandidates(t *testing.T, text string) *candidateParagraph {
	t.Helper()
	s, _ := helpers.NewSyntheticStreamingShaper(2)
	p, err := streamCandidateParagraph(context.Background(), preparationParagraph(t, text), helpers.DefaultStyleResolver{}, preparationDefaults(), *newParameters(), KnotCore{MaxW: dimen.Fill}, s, contracts.ShapeBufferOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestShapingStoragePublication(t *testing.T) {
	p := storageCandidates(t, "alpha beta")
	// The synthetic backend rejects features, so add one after shaping to
	// exercise metadata ownership independently of backend feature support.
	p.runs[0].params.shaping.Features = []contracts.ShapeFeature{{Start: 1, End: 3}}
	k, err := NewKhipukamayuq(Config{}).publishCandidateParagraph(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(k.boxRefs) != 4 || k.boxRefs[0] != 1 || k.boxRefs[1] != 0 || k.boxRefs[2] != 2 || k.boxRefs[3] != 0 {
		t.Fatal(k.boxRefs)
	}
	if len(k.shaping.glyphs) != 9 || len(k.shaping.boxes) != 2 || len(k.shaping.contexts) != 1 {
		t.Fatal("incorrect arena sizes")
	}
	for i, c := range p.knots {
		if k.KnotByIndex(i) != c.core {
			t.Fatal("metrics changed")
		}
	}
	box := k.shaping.boxes[1]
	if box.source.byteStart != 6 || box.glyphs.start != 5 || box.glyphs.end != 9 || box.context != 0 {
		t.Fatal(box)
	}
	// Candidate buffers are independent after publication.
	p.knots[0].clusters[0].glyphs[0].GlyphID = 999
	p.runs[0].params.shaping.Features[0].End = 99
	p.boundaries[0].penalty = 42
	if k.shaping.glyphs[0].GlyphID == 999 || k.shaping.contexts[0].params.shaping.Features[0].End != 3 || k.shaping.boundaries[0].penalty == 42 {
		t.Fatal("borrowed mutable storage")
	}
	if k.paragraph != p.paragraph {
		t.Fatal("lost original source")
	}
}

func TestShapingStorageMixedClustersAndGaps(t *testing.T) {
	a, g := candidateFixture(t, "a b")
	knots := accumulateTestGlyphs(t, a, g, []contracts.ShapedGlyph{{Cluster: 0, GlyphID: 1, XAdvance: 500}})
	p := &candidateParagraph{a.para, a.runs, a.boundaries, knots}
	k, err := NewKhipukamayuq(Config{}).publishCandidateParagraph(p)
	if err != nil || len(k.shaping.glyphs) != 1 || k.shaping.boxes[0].source.byteEnd != 3 {
		t.Fatal(k, err)
	}
	p = storageCandidates(t, "a\u2067אב\u2069z")
	k, err = NewKhipukamayuq(Config{}).publishCandidateParagraph(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(k.shaping.contexts) != 3 || k.shaping.boxes[1].source.scalarStart != 2 {
		t.Fatal("lost direction/source context")
	}
	for _, box := range k.shaping.boxes {
		for _, c := range k.shaping.clusters[box.clusters.start:box.clusters.end] {
			if c.glyphs.start < box.glyphs.start || c.glyphs.end > box.glyphs.end {
				t.Fatal("cluster range outside box")
			}
		}
	}
}

func TestShapingStorageEmptyAndLifecycle(t *testing.T) {
	owner := NewKhipukamayuq(Config{})
	for _, text := range []string{"", "  ", "\u2066\u2069"} {
		k, err := owner.publishCandidateParagraph(storageCandidates(t, text))
		if err != nil {
			t.Fatal(err)
		}
		if len(k.shaping.boxes) != 0 || len(k.shaping.glyphs) != 0 {
			t.Fatal("stored glue glyphs")
		}
	}
	k, err := owner.publishCandidateParagraph(storageCandidates(t, "a"))
	if err != nil {
		t.Fatal(err)
	}
	k.appendKnot(KnotCore{Kind: KTGlue, Flags: KFDiscardable})
	k.Unskip()
	if len(k.boxRefs) != len(k.Kind) || len(k.shaping.sources) != len(k.Kind) {
		t.Fatal("association planes misaligned")
	}
	k.Unskip() // mandatory fill is retained
	if len(k.Kind) != 2 {
		t.Fatal("removed fill")
	}
	unwireKhipu(k)
	if k.boxRefs != nil || k.shaping.glyphs != nil || k.shaping.contexts != nil {
		t.Fatal("arena retained on unwire")
	}
	legacy, err := owner.EncodeParagraph(preparationParagraph(t, "abc"), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range legacy.boxRefs {
		if id != 0 {
			t.Fatal("legacy glyph association")
		}
	}
}

func TestShapingStorageRejectsInvalidCandidates(t *testing.T) {
	p := storageCandidates(t, "abc")
	p.knots[0].clusters[0].runIndex = 99
	if k, err := NewKhipukamayuq(Config{}).publishCandidateParagraph(p); k != nil || err == nil {
		t.Fatal("published partial storage")
	}
}
