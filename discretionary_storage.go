package khipu

import (
	"fmt"
	"math"
	"slices"
	"unicode/utf8"

	"github.com/npillmayer/khipu/contracts"
)

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
		!fragment.params.shaping.Font.Valid() {
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
	if err := width.add(fragment.glyphs, fragment.params.shaping.Font.Scale()); err != nil {
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
// Staged records remain immutable and reusable by other plans/freezes.
func (a *shapingArena) appendReplacement(staged shapingArena) shapedBoxID {
	box := staged.boxes[0]
	box.context = len(a.contexts)
	box.glyphs.start += len(a.glyphs)
	box.glyphs.end += len(a.glyphs)
	box.clusters.start += len(a.clusters)
	box.clusters.end += len(a.clusters)
	staged.clusters = slices.Clone(staged.clusters)
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
