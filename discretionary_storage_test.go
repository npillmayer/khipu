package khipu

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/helpers"
)

func discretionaryStorageFixture(t *testing.T) (*Khipu, *shapedDiscretionaryInput) {
	t.Helper()
	k, err := NewKhipukamayuq(Config{}).publishCandidateParagraph(storageCandidates(t, "aéz"))
	if err != nil {
		t.Fatal(err)
	}
	in, err := prepareDiscretionaryInput("aéz", k.shaping.sources[0].source, HyphenationFragment{
		PreBreak: "aé-", PostBreak: "z", SourceSplit: SourceSplit{3, true},
	})
	if err != nil {
		t.Fatal(err)
	}
	params := k.shaping.contexts[0].params
	pair, err := shapeDiscretionaryInput(context.Background(), in, params, params, func() (contracts.StreamingShaper, error) {
		return helpers.NewSyntheticStreamingShaper(0)
	}, contracts.ShapeBufferOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return k, pair
}

func TestDiscretionaryStoragePublication(t *testing.T) {
	k, pair := discretionaryStorageFixture(t)
	k.paragraph.Offset = 90
	pair.pre.params.shaping.Features = []contracts.ShapeFeature{{Enabled: true}}
	base := k.KnotByIndex(0)
	baseBox := k.shaping.boxes[0]
	cand, err := k.publishDiscretionary(0, 1, 37, pair)
	if err != nil {
		t.Fatal(err)
	}
	if cand.preBox != 2 || cand.postBox != 3 || cand.PreBreak.Penalty != 37 || cand.PostBreak.Penalty != 0 || cand.SourceSplit != pair.split {
		t.Fatal(cand)
	}
	if cand.PreBreak.Pos != 90 || cand.PreBreak.Len != 3 || cand.PostBreak.Pos != 93 || cand.PostBreak.Len != 1 {
		t.Fatal("fragment core does not describe original source", cand)
	}
	if k.KnotByIndex(0) != base || k.shaping.boxes[0] != baseBox || k.boxRefs[0] != 1 {
		t.Fatal("base paragraph changed")
	}
	pre := k.shaping.boxes[cand.preBox-1]
	if pre.replacement == nil || *pre.replacement != pair.pre.input || pre.source != pair.pre.input.provenance {
		t.Fatal("missing replacement/provenance", pre)
	}
	if !reflect.DeepEqual(k.shaping.glyphs[pre.glyphs.start:pre.glyphs.end], pair.pre.glyphs) {
		t.Fatal("glyphs changed")
	}
	clusters := k.shaping.clusters[pre.clusters.start:pre.clusters.end]
	if len(clusters) != 3 || clusters[1].source != (runSourceRange{1, 3, 1, 2}) || clusters[2].source != (runSourceRange{3, 4, 2, 3}) {
		t.Fatal("replacement clusters use wrong coordinates", clusters)
	}
	for _, c := range clusters {
		if c.glyphs.start < pre.glyphs.start || c.glyphs.end > pre.glyphs.end {
			t.Fatal("unrelocated glyph range")
		}
	}
	context := k.shaping.contexts[pre.context]
	if context.source != (runSourceRange{0, 4, 0, 3}) {
		t.Fatal("context not replacement-local")
	}
	pair.pre.glyphs[0].GlyphID = 999
	pair.pre.params.shaping.Features[0].Enabled = false
	pair.pre.input.text = "changed"
	if k.shaping.glyphs[pre.glyphs.start].GlyphID == 999 || !context.params.shaping.Features[0].Enabled || pre.replacement.text != "aé-" {
		t.Fatal("borrowed fragment storage")
	}
	if !k.SelectDiscretionary(0, DiscretionarySelection{Source: 0, Variant: 1}) {
		t.Fatal("selection failed")
	}
	choice, _ := k.SelectedDiscretionaryAt(0)
	if k.DiscretionariesAt(choice.Source)[0] != cand {
		t.Fatal("selection lost box refs")
	}
	// Force arena growth using independent valid alternatives. Existing records
	// and IDs must remain usable without rebuilding ordinary box associations.
	_, fresh := discretionaryStorageFixture(t)
	for variant := uint16(2); variant < 30; variant++ {
		if _, err := k.publishDiscretionary(0, variant, 0, fresh); err != nil {
			t.Fatal(err)
		}
	}
	if k.shaping.boxes[cand.preBox-1] != pre || k.DiscretionariesAt(0)[0] != cand {
		t.Fatal("unstable references")
	}
	unwireKhipu(k)
	if k.shaping.boxes != nil || k.shaping.contexts != nil || k.DiscretionaryCandidates != nil {
		t.Fatal("retained arena after unwire")
	}
}

func TestDiscretionaryStorageFailureIsAtomic(t *testing.T) {
	for _, mode := range []string{"source", "split", "partition", "duplicate", "post glyph", "post width", "post feature", "index", "kind"} {
		t.Run(mode, func(t *testing.T) {
			k, pair := discretionaryStorageFixture(t)
			if _, err := k.publishDiscretionary(0, 1, 0, pair); err != nil {
				t.Fatal(err)
			}
			variant, at := uint16(2), 0
			switch mode {
			case "source":
				pair.source.byteEnd++
			case "split":
				pair.split.Valid = false
			case "partition":
				pair.post.input.provenance.scalarStart++
			case "duplicate":
				variant = 1
			case "post glyph":
				pair.post.glyphs[0].Cluster = 99
			case "post width":
				pair.post.widths.W++
			case "post feature":
				pair.post.params.shaping.Features = []contracts.ShapeFeature{{End: 1}}
			case "index":
				at = 99
			case "kind":
				at = 1
			}
			before := k.shaping
			before.boxes = slices.Clone(before.boxes)
			before.glyphs = slices.Clone(before.glyphs)
			before.clusters = slices.Clone(before.clusters)
			before.contexts = slices.Clone(before.contexts)
			cached := slices.Clone(k.DiscretionariesAt(0))
			got, err := k.publishDiscretionary(at, variant, 0, pair)
			if !errors.Is(err, contracts.ErrIllegalArguments) || got != (DiscretionaryCandidate{}) {
				t.Fatal(got, err)
			}
			if !reflect.DeepEqual(before, k.shaping) || !reflect.DeepEqual(cached, k.DiscretionariesAt(0)) {
				t.Fatal("partial publication")
			}
		})
	}
}

func TestDiscretionaryStorageEmptyAndMetricCandidates(t *testing.T) {
	k, pair := discretionaryStorageFixture(t)
	pair.pre.input.text, pair.pre.input.scalarCount = "", 0
	pair.pre.glyphs, pair.pre.widths = nil, candidateWidths{}
	// Nonempty input may also produce no glyphs; its source must not vanish.
	pair.post.glyphs, pair.post.widths = nil, candidateWidths{}
	cand, err := k.publishDiscretionary(0, 1, 0, pair)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []shapedBoxID{cand.preBox, cand.postBox} {
		box := k.shaping.boxes[id-1]
		if box.replacement == nil || box.glyphs.start != box.glyphs.end || box.clusters.start != box.clusters.end {
			t.Fatal(box)
		}
	}
	metric := DiscretionaryCandidate{Variant: 2, PreBreak: KnotCore{W: 10}, PostBreak: KnotCore{W: 20}}
	if !k.AddDiscretionaryCandidate(0, metric) || k.DiscretionariesAt(0)[1] != metric || metric.preBox != 0 || metric.postBox != 0 {
		t.Fatal("metric-only candidates changed")
	}
}
