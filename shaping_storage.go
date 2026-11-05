package khipu

import (
	"fmt"
	"slices"

	"github.com/npillmayer/khipu/contracts"
)

// shapedBoxID is a one-based index in the owning khipu's arena; zero is absent.
// IDs, not pointers into slices, survive arena growth. They are owner-local.
type shapedBoxID int

type arenaRange struct{ start, end int }

// shapedBox retains ordinary text shaping. source is original paragraph text
// and, for this first storage path, also the shaping input. Generated replacement
// text for discretionary fragments will need a distinct input representation.
type shapedBox struct {
	source   runSourceRange
	context  int
	glyphs   arenaRange
	clusters arenaRange
}

type storedGlyphCluster struct {
	source runSourceRange
	glyphs arenaRange
}

// knotSource preserves provenance separately from geometry. A break in omitted
// text can be beyond the glyph-bearing span. Glue has provenance but no box ID.
type knotSource struct {
	source     runSourceRange
	breakAfter sourceBoundary
}

// shapingArena owns immutable glyphs, clusters and feature slices. Contexts
// retain original prepared-run ranges: feature coordinates are relative to that
// run, not a particular box. Reshaping a box must clip and rebase those features.
// Font faces/scales and the paragraph are shared under their stable-lifetime
// contracts. Appending new records must never mutate published records. No
// renderer API, discretionary migration or arena compaction is introduced here.
type shapingArena struct {
	boxes      []shapedBox
	glyphs     []contracts.ShapedGlyph
	clusters   []storedGlyphCluster
	contexts   []preparedRun
	sources    []knotSource
	boundaries []sourceBoundary
}

// publishCandidateParagraph copies successful internal candidates into a new
// owned Khipu. It is not called by EncodeParagraph yet. The candidate result may
// be discarded or its slices reused afterward; the original styled paragraph
// must remain stable. Failure returns no partially published khipu.
func (kq *Khipukamayuq) publishCandidateParagraph(p *candidateParagraph) (*Khipu, error) {
	if kq == nil || p == nil || p.paragraph == nil {
		return nil, fmt.Errorf("%w: missing candidate owner or paragraph", contracts.ErrIllegalArguments)
	}
	if _, err := newGlyphClusterAssembler(p.runs, p.boundaries); err != nil {
		return nil, err
	}
	k := kq.allocKhipu(len(p.knots))
	k.paragraph = p.paragraph
	k.shaping.boundaries = slices.Clone(p.boundaries)
	k.shaping.sources = make([]knotSource, 0, len(p.knots))
	contexts := make(map[int]int)
	bad := func() (*Khipu, error) {
		return nil, fmt.Errorf("%w: inconsistent candidate shaping data", contracts.ErrIllegalArguments)
	}
	for _, candidate := range p.knots {
		k.appendKnot(candidate.core)
		k.shaping.sources[len(k.shaping.sources)-1] = knotSource{candidate.source, candidate.breakAfter}
		if candidate.core.Kind != KTTextBox {
			continue
		}
		if len(candidate.clusters) == 0 {
			return bad()
		}
		runID := candidate.clusters[0].runIndex
		if runID < 0 || runID >= len(p.runs) {
			return bad()
		}
		context, ok := contexts[runID]
		if !ok {
			context = len(k.shaping.contexts)
			run := p.runs[runID]
			run.params.shaping.Features = slices.Clone(run.params.shaping.Features)
			k.shaping.contexts = append(k.shaping.contexts, run)
			contexts[runID] = context
		}
		box := shapedBox{source: candidate.source, context: context,
			glyphs: arenaRange{start: len(k.shaping.glyphs)}, clusters: arenaRange{start: len(k.shaping.clusters)}}
		end := candidate.source.scalarStart
		for _, c := range candidate.clusters {
			if c.runIndex != runID || c.source.scalarStart != end || c.source.scalarEnd <= end || len(c.glyphs) == 0 {
				return bad()
			}
			start := len(k.shaping.glyphs)
			k.shaping.glyphs = append(k.shaping.glyphs, c.glyphs...)
			k.shaping.clusters = append(k.shaping.clusters, storedGlyphCluster{c.source, arenaRange{start, len(k.shaping.glyphs)}})
			end = c.source.scalarEnd
		}
		if end != candidate.source.scalarEnd {
			return bad()
		}
		box.glyphs.end = len(k.shaping.glyphs)
		box.clusters.end = len(k.shaping.clusters)
		k.shaping.boxes = append(k.shaping.boxes, box)
		k.boxRefs[len(k.boxRefs)-1] = shapedBoxID(len(k.shaping.boxes))
	}
	return k, nil
}
