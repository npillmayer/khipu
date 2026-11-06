package khipu

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
	"github.com/npillmayer/uax/bidi"
)

func TestStreamingSoftHyphenNoBreak(t *testing.T) {
	for _, text := range []string{"co\u00adoperate", "é\u00adß\u00ad文", "\u00adabc", "abc\u00ad", "\u00ad\u00ad", "a\u00ad\u2067אב\u00adג\u2069z", "a \u00adb"} {
		t.Run(text, func(t *testing.T) {
			owner := NewKhipukamayuq(Config{Streaming: &StreamingConfig{
				Resolver: helpers.DefaultStyleResolver{}, Defaults: preparationDefaults(),
				NewShaper: func() (contracts.StreamingShaper, error) { return helpers.NewSyntheticStreamingShaper(0) },
			}})
			p := preparationParagraph(t, text)
			k, err := owner.EncodeParagraph(context.Background(), p)
			if err != nil {
				t.Fatal(err)
			}
			plain, err := owner.EncodeParagraph(context.Background(), preparationParagraph(t, strings.ReplaceAll(text, softHyphenMarker, "")))
			if err != nil {
				t.Fatal(err)
			}
			var w, pw dimen.DU
			for _, v := range k.W {
				w += v
			}
			for _, v := range plain.W {
				pw += v
			}
			if w != pw {
				t.Fatalf("marker contributed width: %v != %v", w, pw)
			}
			runes := []rune(text)
			for _, g := range k.shaping.glyphs {
				if int(g.Cluster) >= len(runes) || runes[g.Cluster] == '\u00ad' || g.GlyphID != uint32(runes[g.Cluster])+1 {
					t.Fatalf("lost original anchor: %+v", g)
				}
			}
			markers := 0
			for _, b := range k.shaping.boundaries {
				if b.softHyphen {
					markers++
					if b.penalty != MaxPenalty || b.wordBoundary || runes[b.scalarEnd-1] != '\u00ad' {
						t.Fatal("ordinary marker break retained", b)
					}
					if text[b.byteEnd-2:b.byteEnd] != softHyphenMarker {
						t.Fatal("lost marker bytes")
					}
				}
			}
			if markers != strings.Count(text, softHyphenMarker) {
				t.Fatal("lost opportunities")
			}
			if k.Pos[len(k.Pos)-1] != uint64(len(text)) {
				t.Fatal("lost original paragraph length")
			}
			if len(k.DiscretionaryCandidates) != 0 || len(k.SelectedDiscretionaries) != 0 {
				t.Fatal("premature candidate discovery")
			}
			if text == "co\u00adoperate" && (len(k.Kind) != 2 || k.Penalty[0] != MaxPenalty || k.Len[0] != uint16(len(text))) {
				t.Fatal("marker split ordinary textbox", k.Kind, k.Penalty)
			}
		})
	}
}

func TestNoBreakClusterSpansMarker(t *testing.T) {
	s := &fragmentRecordingShaper{pipelineShaper: newPipelineShaper(), glyphs: []contracts.ShapedGlyph{
		{Cluster: 0, GlyphID: 123, XAdvance: 500}, {Cluster: 2, GlyphID: 456, XAdvance: 500},
	}}
	// Model a ligature over f+i; x is the next filtered-input scalar.
	p, err := streamCandidateParagraph(context.Background(), preparationParagraph(t, "f\u00adix"), helpers.DefaultStyleResolver{}, preparationDefaults(), *newParameters(), KnotCore{MaxW: dimen.Fill}, s, contracts.ShapeBufferOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if s.text != "fix" || len(p.knots) != 2 || len(p.knots[0].clusters) != 2 {
		t.Fatal("incorrect no-break shaping", s.text, p.knots)
	}
	first := p.knots[0].clusters[0]
	if first.source != (runSourceRange{0, 4, 0, 3}) || p.knots[0].clusters[1].glyphs[0].Cluster != 3 {
		t.Fatal("incorrect ligature provenance", first)
	}
	if !p.boundaries[1].softHyphen {
		t.Fatal("lost internal opportunity")
	}
}

func TestNoBreakFeatureCoordinates(t *testing.T) {
	d := preparationDefaults()
	d.Features = []contracts.ShapeFeature{{Start: 0, End: 4, Value: 1}, {Start: 1, End: 2, Value: 2}, {Start: 2, End: 4, Value: 3}}
	s := &fragmentRecordingShaper{pipelineShaper: newPipelineShaper(), glyphs: []contracts.ShapedGlyph{{Cluster: 0, XAdvance: 500}}}
	p, err := streamCandidateParagraph(context.Background(), preparationParagraph(t, "a\u00adbc"), helpers.DefaultStyleResolver{}, d, *newParameters(), KnotCore{MaxW: dimen.Fill}, s, contracts.ShapeBufferOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := []contracts.ShapeFeature{{Start: 0, End: 3, Value: 1}, {Start: 1, End: 3, Value: 3}}
	if !reflect.DeepEqual(s.params.Features, want) || !reflect.DeepEqual(p.runs[0].params.shaping.Features, d.Features) {
		t.Fatal("feature mapping/retention", s.params.Features, p.runs[0].params.shaping.Features)
	}
}

func TestNoBreakInvalidBackendAnchor(t *testing.T) {
	s := &fragmentRecordingShaper{pipelineShaper: newPipelineShaper(), glyphs: []contracts.ShapedGlyph{{Cluster: 2}}}
	p, err := streamCandidateParagraph(context.Background(), preparationParagraph(t, "a\u00adb"), helpers.DefaultStyleResolver{}, preparationDefaults(), *newParameters(), KnotCore{MaxW: dimen.Fill}, s, contracts.ShapeBufferOptions{})
	if p != nil || !errors.Is(err, contracts.ErrIllegalArguments) {
		t.Fatal("accepted anchor beyond filtered input", p, err)
	}
}

func TestNoBreakPreservesOtherBoundaries(t *testing.T) {
	for _, text := range []string{"a \u00adb", "a\n\u00adb"} {
		p := storageCandidates(t, text)
		found := false
		for _, k := range p.knots {
			if k.breakAfter.byteEnd == 2 && k.core.Penalty < MaxPenalty {
				found = true
				if strings.Contains(text, "\n") && k.core.Penalty != MaxMerit {
					t.Fatal("lost mandatory break")
				}
			}
		}
		if !found {
			t.Fatal("lost pre-marker ordinary break", text, p.knots)
		}
	}
}

func TestNoBreakCoalescingPreservesStyleBoundaries(t *testing.T) {
	text := styled.TextFromString("co\u00adoperate")
	text, err := text.Style(intersectionStyle("bold"), 4, 7)
	if err != nil {
		t.Fatal(err)
	}
	para, err := styled.ParagraphFromText(&text, 0, 11, bidi.LeftToRight, nil)
	if err != nil {
		t.Fatal(err)
	}
	items, err := intersectParagraphRuns(para)
	if err != nil {
		t.Fatal(err)
	}
	boundaries, err := prepareParagraphBoundaries(para)
	if err != nil {
		t.Fatal(err)
	}
	got := coalesceMarkerIntersections(items, boundaries)
	if len(got) < 2 {
		t.Fatal("merged styled text")
	}
	for _, item := range got {
		if item.source.byteStart < 4 && item.source.byteEnd > 4 {
			t.Fatal("crossed style boundary", item)
		}
	}
}
