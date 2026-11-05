package khipu

import (
	"fmt"
	"math"
	"slices"
	"unicode/utf8"

	"github.com/npillmayer/khipu/contracts"
)

// publishDiscretionary stores one fully shaped alternative for one original
// textbox. Discovery and penalty policy remain with its caller. Both fragments
// are prepared privately before any arena/cache mutation; errors leave the
// khipu unchanged. Publication is not concurrent-safe, like the khipu itself.
// Existing variants cannot be replaced: selections and arena IDs remain stable.
// Multi-box replacements and public renderer access are deliberately excluded.
func (k *Khipu) publishDiscretionary(at int, variant uint16, penalty Penalty, pair *shapedDiscretionaryInput) (DiscretionaryCandidate, error) {
	staged, err := k.prepareDiscretionaryPublication(at, variant, penalty, pair)
	if err != nil {
		return DiscretionaryCandidate{}, err
	}
	return k.commitDiscretionary(at, staged), nil
}

// stagedDiscretionary permits a complete provider batch to be validated before
// publishing any alternative. The private arenas own all mutable storage.
type stagedDiscretionary struct {
	candidate DiscretionaryCandidate
	pre, post shapingArena
}

func (k *Khipu) prepareDiscretionaryPublication(at int, variant uint16, penalty Penalty, pair *shapedDiscretionaryInput) (stagedDiscretionary, error) {
	bad := func() (stagedDiscretionary, error) {
		return stagedDiscretionary{}, fmt.Errorf("%w: invalid discretionary publication", contracts.ErrIllegalArguments)
	}
	if k == nil || pair == nil || k.paragraph == nil || at < 0 || at >= len(k.Kind) ||
		k.Kind[at] != KTTextBox || at >= len(k.shaping.sources) || at >= len(k.boxRefs) || k.boxRefs[at] == 0 {
		return bad()
	}
	s, a, b := pair.source, pair.pre.input.provenance, pair.post.input.provenance
	if s != k.shaping.sources[at].source || !pair.split.Valid ||
		a.byteStart != s.byteStart || a.scalarStart != s.scalarStart ||
		b.byteEnd != s.byteEnd || b.scalarEnd != s.scalarEnd ||
		a.byteEnd != pair.split.ByteOffset || b.byteStart != pair.split.ByteOffset || a.scalarEnd != b.scalarStart {
		return bad()
	}
	for _, existing := range k.DiscretionariesAt(at) {
		if existing.Variant == variant {
			return bad()
		}
	}
	pre, preCore, err := prepareStoredReplacement(pair.pre, k.paragraph.Offset)
	if err != nil {
		return stagedDiscretionary{}, err
	}
	post, postCore, err := prepareStoredReplacement(pair.post, k.paragraph.Offset)
	if err != nil {
		return stagedDiscretionary{}, err
	}
	preCore.Penalty = penalty
	candidate := DiscretionaryCandidate{Variant: variant, PreBreak: preCore, PostBreak: postCore, SourceSplit: pair.split}
	return stagedDiscretionary{candidate, pre, post}, nil
}

func (k *Khipu) commitDiscretionary(at int, staged stagedDiscretionary) DiscretionaryCandidate {
	// No fallible work remains. Appending preserves existing IDs and records;
	// only the new local ranges need relocation into the destination arena.
	candidate := staged.candidate
	candidate.preBox = k.shaping.appendReplacement(staged.pre)
	candidate.postBox = k.shaping.appendReplacement(staged.post)
	k.AddDiscretionaryCandidate(at, candidate)
	return candidate
}

// prepareStoredReplacement owns copied glyphs/features and builds cluster ranges
// in replacement coordinates. Silent prefixes stay in replacement.text; final
// cluster coverage extends to input end, just as ordinary cluster assembly does.
// KnotCore.Pos/Len describe ORIGINAL provenance, not generated spelling. Clients
// must use SourceSplit and retained replacement input rather than infer either
// from a fragment's length. Empty fragments still get a distinct arena box.
func prepareStoredReplacement(fragment shapedReplacement, offset uint64) (shapingArena, KnotCore, error) {
	bad := func() (shapingArena, KnotCore, error) {
		return shapingArena{}, KnotCore{}, fmt.Errorf("%w: invalid shaped replacement", contracts.ErrIllegalArguments)
	}
	in := fragment.input
	s := in.provenance
	if !utf8.ValidString(in.text) || in.scalarCount != uint64(utf8.RuneCountInString(in.text)) ||
		in.scalarCount > uint64(math.MaxUint32)+1 || s.byteStart > s.byteEnd || s.scalarStart > s.scalarEnd ||
		s.byteEnd-s.byteStart > math.MaxUint16 || offset > math.MaxUint64-s.byteEnd ||
		nilRunService(fragment.params.shaping.Font) || nilRunService(fragment.params.scale) || fragment.params.emSize <= 0 {
		return bad()
	}
	for _, f := range fragment.params.shaping.Features {
		if f.Start > 0 || f.End > 0 {
			return bad()
		}
	}
	var width candidateWidth
	for i, glyph := range fragment.glyphs {
		if uint64(glyph.Cluster) >= in.scalarCount || (i > 0 && glyph.Cluster < fragment.glyphs[i-1].Cluster) {
			return bad()
		}
	}
	if err := width.add(fragment.glyphs, fragment.params.scale); err != nil {
		return bad()
	}
	w, err := width.finish(candidateText, contracts.SpacingFixed, Params{})
	if err != nil || w != fragment.widths {
		return bad()
	}
	params := fragment.params
	params.shaping.Features = slices.Clone(params.shaping.Features)
	local := runSourceRange{0, uint64(len(in.text)), 0, in.scalarCount}
	arena := shapingArena{glyphs: slices.Clone(fragment.glyphs), contexts: []preparedRun{{source: local, params: params}}}
	byteOffsets := make([]uint64, 0, in.scalarCount+1)
	for pos := range in.text {
		byteOffsets = append(byteOffsets, uint64(pos))
	}
	byteOffsets = append(byteOffsets, uint64(len(in.text)))
	for start := 0; start < len(arena.glyphs); {
		end := start + 1
		anchor := arena.glyphs[start].Cluster
		for end < len(arena.glyphs) && arena.glyphs[end].Cluster == anchor {
			end++
		}
		limit := in.scalarCount
		if end < len(arena.glyphs) {
			limit = uint64(arena.glyphs[end].Cluster)
		}
		arena.clusters = append(arena.clusters, storedGlyphCluster{
			source: runSourceRange{byteOffsets[anchor], byteOffsets[limit], uint64(anchor), limit}, glyphs: arenaRange{start, end},
		})
		start = end
	}
	arena.boxes = []shapedBox{{source: s, context: 0, glyphs: arenaRange{0, len(arena.glyphs)}, clusters: arenaRange{0, len(arena.clusters)}, replacement: &in}}
	core := KnotCore{Pos: offset + s.byteStart, Len: uint16(s.byteEnd - s.byteStart), Kind: KTTextBox, W: w.W, MinW: w.MinW, MaxW: w.MaxW}
	return arena, core, nil
}

// appendReplacement relocates a private single-box arena after validation.
// The staged records are owned and no longer used by the caller afterward.
func (a *shapingArena) appendReplacement(staged shapingArena) shapedBoxID {
	box := staged.boxes[0]
	box.context = len(a.contexts)
	box.glyphs.start += len(a.glyphs)
	box.glyphs.end += len(a.glyphs)
	box.clusters.start += len(a.clusters)
	box.clusters.end += len(a.clusters)
	for i := range staged.clusters {
		staged.clusters[i].glyphs.start += len(a.glyphs)
		staged.clusters[i].glyphs.end += len(a.glyphs)
	}
	a.glyphs = append(a.glyphs, staged.glyphs...)
	a.clusters = append(a.clusters, staged.clusters...)
	a.contexts = append(a.contexts, staged.contexts...)
	a.boxes = append(a.boxes, box)
	return shapedBoxID(len(a.boxes))
}
