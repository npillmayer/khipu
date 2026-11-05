package khipu

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/helpers"
)

func clusterFixture(t *testing.T, text string) *glyphClusterAssembler {
	t.Helper()
	p := preparationParagraph(t, text)
	runs, err := prepareParagraphRuns(p, helpers.DefaultStyleResolver{}, preparationDefaults())
	if err != nil {
		t.Fatal(err)
	}
	boundaries, err := prepareParagraphBoundaries(p)
	if err != nil {
		t.Fatal(err)
	}
	a, err := newGlyphClusterAssembler(runs, boundaries)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestGlyphClusterAssembly(t *testing.T) {
	a := clusterFixture(t, "aéz")
	first := contracts.ShapedGlyph{Cluster: 0, GlyphID: 11, XAdvance: 500}
	mark := contracts.ShapedGlyph{Cluster: 0, GlyphID: 12, XOffset: -20, YOffset: 30}
	for _, g := range []contracts.ShapedGlyph{first, mark} {
		if c, err := a.push(g); c != nil || err != nil {
			t.Fatalf("premature cluster %v %v", c, err)
		}
	}
	// Anchor 2 implies a two-scalar first cluster, not two glyph-sized spans.
	c, err := a.push(contracts.ShapedGlyph{Cluster: 2, GlyphID: 13})
	if err != nil {
		t.Fatal(err)
	}
	if c.source != (runSourceRange{0, 3, 0, 2}) || !reflect.DeepEqual(c.glyphs, []contracts.ShapedGlyph{first, mark}) || c.penalty != a.boundaries[1].penalty {
		t.Fatalf("cluster %+v", c)
	}
	last, err := a.finish()
	if err != nil || last.source != (runSourceRange{3, 4, 2, 3}) {
		t.Fatalf("last %v %v", last, err)
	}
	last.glyphs[0].GlyphID = 99
	if c.glyphs[0] != first {
		t.Fatal("shared emitted glyph storage")
	}
	if c, err := a.finish(); c != nil || err != nil {
		t.Fatal("finish not idempotent")
	}
	if _, err := a.push(first); err == nil {
		t.Fatal("accepted glyph after finish")
	}
}

func TestGlyphClusterRunGapsAndSilence(t *testing.T) {
	a := clusterFixture(t, "a\u2067אב\u2069z")
	if c, err := a.push(contracts.ShapedGlyph{Cluster: 0}); c != nil || err != nil {
		t.Fatal(c, err)
	}
	// Skip the entire RTL run. The first cluster must end before its controls.
	c, err := a.push(contracts.ShapedGlyph{Cluster: 5})
	if err != nil || c.source != (runSourceRange{0, 1, 0, 1}) || c.runIndex != 0 {
		t.Fatal(c, err)
	}
	c, err = a.finish()
	if err != nil || c.source != (runSourceRange{11, 12, 5, 6}) || c.runIndex != 2 {
		t.Fatal(c, err)
	}
	a = clusterFixture(t, "abc")
	if _, err := a.push(contracts.ShapedGlyph{Cluster: 1}); err != nil {
		t.Fatal(err)
	}
	c, err = a.finish()
	if err != nil || c.source != (runSourceRange{1, 3, 1, 3}) {
		t.Fatal("leading omitted scalar incorrectly claimed", c, err)
	}
	for _, text := range []string{"", "abc", "\u2066\u2069"} {
		a = clusterFixture(t, text)
		if c, err := a.finish(); c != nil || err != nil {
			t.Fatal("fabricated cluster", c, err)
		}
	}
}

func TestGlyphClusterInvalidAnchors(t *testing.T) {
	for _, tc := range []struct {
		text    string
		anchors []uint32
	}{
		{"abc", []uint32{1, 0}}, {"abc", []uint32{3}},
		{"a\u2067אב\u2069z", []uint32{0, 1}}, {"", []uint32{0}},
	} {
		a := clusterFixture(t, tc.text)
		var err error
		for _, anchor := range tc.anchors {
			_, err = a.push(contracts.ShapedGlyph{Cluster: anchor})
		}
		if !errors.Is(err, contracts.ErrIllegalArguments) {
			t.Fatal("accepted bad anchors", tc, err)
		}
		if c, endErr := a.finish(); c != nil || endErr != err {
			t.Fatal("error not sticky", c, endErr)
		}
	}
}

func TestGlyphClusterMetadataValidation(t *testing.T) {
	a := clusterFixture(t, "é")
	bad := append([]sourceBoundary(nil), a.boundaries...)
	bad[0].scalarEnd = 2
	if _, err := newGlyphClusterAssembler(a.runs, bad); err == nil {
		t.Fatal("accepted noncontiguous scalar table")
	}
	runs := append([]preparedRun(nil), a.runs...)
	runs[0].source.byteEnd = 1
	if _, err := newGlyphClusterAssembler(runs, a.boundaries); err == nil {
		t.Fatal("accepted inconsistent bytes")
	}
}

func TestGlyphClustersFromStreamingProducer(t *testing.T) {
	p := preparationParagraph(t, "a\u2067אב\u2069z")
	runs, err := prepareParagraphRuns(p, helpers.DefaultStyleResolver{}, preparationDefaults())
	if err != nil {
		t.Fatal(err)
	}
	boundaries, err := prepareParagraphBoundaries(p)
	if err != nil {
		t.Fatal(err)
	}
	a, err := newGlyphClusterAssembler(runs, boundaries)
	if err != nil {
		t.Fatal(err)
	}
	s, _ := helpers.NewSyntheticStreamingShaper(1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- producePreparedRuns(ctx, p, runs, s, contracts.ShapeBufferOptions{}) }()
	output := s.Glyphs()
	var clusters []*glyphCluster
	for output != nil || done != nil {
		select {
		case g, ok := <-output:
			if !ok {
				output = nil
				continue
			}
			c, err := a.push(g)
			if err != nil {
				cancel()
				if done != nil {
					<-done
				}
				t.Fatal(err)
			}
			if c != nil {
				clusters = append(clusters, c)
			}
		case err := <-done:
			done = nil
			if err != nil {
				t.Fatal(err)
			}
		case <-ctx.Done():
			if done != nil {
				<-done
			}
			t.Fatal(ctx.Err())
		}
	}
	if err := s.Err(); err != nil {
		t.Fatal(err)
	}
	c, err := a.finish()
	if err != nil {
		t.Fatal(err)
	}
	if c != nil {
		clusters = append(clusters, c)
	}
	want := []runSourceRange{{0, 1, 0, 1}, {4, 6, 2, 3}, {6, 8, 3, 4}, {11, 12, 5, 6}}
	if len(clusters) != len(want) {
		t.Fatal(clusters)
	}
	for i, c := range clusters {
		if c.source != want[i] {
			t.Fatalf("cluster %d: %+v", i, c)
		}
	}
}
