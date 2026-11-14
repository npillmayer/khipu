package khipu

import (
	"fmt"
	"slices"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/contracts"
)

// shapedBoxID is a one-based index in the owning khipu's arena; zero is absent.
// IDs, not pointers into slices, survive arena growth. They are owner-local.
type shapedBoxID int

type arenaRange struct{ start, end int }

// shapedBox.source always describes original paragraph provenance. With nil
// replacement it also identifies shaping input. Otherwise replacement retains
// distinct generated input (including an empty string); glyph/cluster coordinates
// and the associated context's source range are local to that replacement.
type shapedBox struct {
	source      runSourceRange
	context     int
	glyphs      arenaRange
	clusters    arenaRange
	replacement *replacementInput
}

// source uses original paragraph coordinates for ordinary boxes and local
// replacement-input coordinates for replacement boxes. It is not a per-glyph
// mapping from generated text back to the original paragraph.
type storedGlyphCluster struct {
	source runSourceRange
	glyphs arenaRange
}

// knotSource preserves provenance separately from geometry. A break in omitted
// text can be beyond the glyph-bearing span. Glue has provenance but no box ID.
type knotSource struct {
	atomic     contracts.AtomicID
	source     runSourceRange
	breakAfter sourceBoundary
}

// shapingArena owns immutable glyphs, clusters and feature slices. Contexts
// retain original prepared-run ranges for ordinary boxes, and local input ranges
// for replacements. Feature coordinates refer to that context, not provenance.
// Reshaping an ordinary box must clip and rebase its run's features.
// Font faces/scales and the paragraph are shared under their stable-lifetime
// contracts. Appending new records must never mutate published records. No
// renderer API or arena compaction is introduced here.
type shapingArena struct {
	boxes      []shapedBox
	glyphs     []contracts.ShapedGlyph
	clusters   []storedGlyphCluster
	contexts   []preparedRun
	sources    []knotSource
	boundaries []sourceBoundary
}

// preparedMaterial stores compiled knot templates and shaping data. Atomic
// templates are unbound slots, never a working Khipu with fabricated advances.
type preparedMaterial struct {
	paragraph *styled.Paragraph
	knots     []KnotCore
	boxRefs   []shapedBoxID
	shaping   shapingArena
}

// compileCandidateParagraph copies successful internal candidates into owned
// immutable material at the end of preparation. The candidate result may
// be discarded or its slices reused afterward; the original styled paragraph
// must remain stable. Failure returns no partially published khipu.
func compileCandidateParagraph(p *candidateParagraph) (*preparedMaterial, error) {
	if p == nil || p.paragraph == nil {
		return nil, fmt.Errorf("%w: missing candidate owner or paragraph", contracts.ErrIllegalArguments)
	}
	if _, err := newGlyphClusterAssembler(p.runs, p.boundaries); err != nil {
		return nil, err
	}
	k := &preparedMaterial{paragraph: p.paragraph, knots: make([]KnotCore, 0, len(p.knots)), boxRefs: make([]shapedBoxID, 0, len(p.knots))}
	k.shaping.boundaries = slices.Clone(p.boundaries)
	k.shaping.sources = make([]knotSource, 0, len(p.knots))
	contexts := make(map[int]int)
	bad := func() (*preparedMaterial, error) {
		return nil, fmt.Errorf("%w: inconsistent candidate shaping data", contracts.ErrIllegalArguments)
	}
	for _, candidate := range p.knots {
		k.knots = append(k.knots, candidate.core)
		k.boxRefs = append(k.boxRefs, 0)
		k.shaping.sources = append(k.shaping.sources, knotSource{atomic: candidate.atomic, source: candidate.source, breakAfter: candidate.breakAfter})
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

// publishCandidateParagraph preserves the text-only EncodeParagraph path.
func (kq *Khipukamayuq) publishCandidateParagraph(p *candidateParagraph) (*Khipu, error) {
	if kq == nil {
		return nil, ErrVoidKhipukamayuq
	}
	material, err := compileCandidateParagraph(p)
	if err != nil {
		return nil, err
	}
	k := kq.allocKhipu(len(material.knots))
	for _, core := range material.knots {
		k.appendKnot(core)
	}
	k.paragraph, k.shaping, k.boxRefs = material.paragraph, material.shaping, material.boxRefs
	return k, nil
}
