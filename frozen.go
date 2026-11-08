package khipu

import (
	"errors"
	"fmt"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
)

var errUnsupportedAdjustment = errors.New("khipu: unsupported non-glue width adjustment")

// frozenParagraph is the internal final-width representation. Its aligned SOA
// planes concatenate lines in paragraph order, and knots in physical visual
// order within each line. It is deliberately NOT a Khipu: no elastic cores,
// MinW/MaxW, penalties, candidate tables, or operation for unfreezing survive.
// The elastic input is left intact; retaining it is not an unfreeze operation.
//
// Planes, provenance and line records are independently owned and immutable
// after publication. Source text/styles, font services and published shaping
// records are shared under their existing immutable-lifetime contracts. There
// is no dependency on the input's mutable selections or slice headers. This
// internal type does not yet establish a public freeze or renderer API.
type frozenParagraph struct {
	paragraph *styled.Paragraph
	shaping   frozenShaping
	w         []dimen.DU
	pos       []uint64
	len       []uint16
	kind      []KnotType
	flags     []KnotFlags
	origins   []frozenOrigin
	lines     []frozenLine
}

// frozenLine indexes a half-open range in the paragraph's content planes.
// The physical sequence is leftSkip, content, rightSkip in either direction.
// Side skips have no fabricated source identity; paragraph fill remains a
// content knot at its already determined visual edge. width includes both
// skips, while the byte interval describes untrimmed original source coverage.
type frozenLine struct {
	content             arenaRange
	leftSkip, rightSkip dimen.DU
	width               dimen.DU
	byteStart, byteEnd  uint64
}

// frozenOrigin preserves the association with original source and chosen
// replacement material, not alternative break opportunities. Pos/Len alone
// cannot identify changed-spelling or silent discretionary replacements.
type frozenOrigin struct {
	source        runSourceRange
	box           shapedBoxID
	origin        int
	variant       uint16
	fragment      logicalFragment
	paragraphFill bool
}

// frozenShaping retains immutable shaping records with their existing IDs.
// Unlike shapingArena, it has no knot-source/boundary tables carrying break
// penalties. Only box IDs in origins are applied output; unused arena records
// may remain reachable until a future compaction step. Glyphs and their order
// inside each box are unchanged, including selected replacement glyphs.
type frozenShaping struct {
	boxes    []shapedBox
	glyphs   []contracts.ShapedGlyph
	clusters []storedGlyphCluster
	contexts []preparedRun
}

// freezeParagraph connects complete break decisions to final-width output.
// Use the same shape and side skips as the breaker, before its selections are
// replaced. Materialization validates ownership/provenance, trims, substitutes
// selected fragments and orders knots; geometry snapshots inclusive targets.
// No shaper or hyphenator is consulted. Failure publishes no partial paragraph
// and leaves the elastic input and its decisions unchanged.
func (kq *Khipukamayuq) freezeParagraph(k *Khipu, breaks []int, params Params) (*frozenParagraph, error) {
	p, err := kq.materializeLineGeometry(k, breaks, params)
	if err != nil {
		return nil, err
	}
	return p.freeze()
}

// freeze realizes the currently supported subset: only glue can change width.
// Every non-glue content knot must declare MinW == W == MaxW, even if this
// target needs no adjustment or a higher-order fill would absorb it all.
// Advertising textbox capacity without a target-width shaping service is not
// enough to realize it: changing W alone would leave stale glyph metrics.
// Fixed kern/custom material is preserved; elastic non-glue material is rejected.
// Side skips are edge glue by role, independent of their incidental Kind.
//
// Each line uses the allocator's exact integer assignment (including fill
// precedence and bounded shrink). All output remains private until every line
// succeeds. Geometry and immutable shaping storage are never modified.
func (p *geometryParagraph) freeze() (*frozenParagraph, error) {
	if p == nil || p.paragraph == nil || len(p.lines) == 0 {
		return nil, fmt.Errorf("%w: freeze requires prepared geometry", ErrIllegalArguments)
	}
	result := &frozenParagraph{paragraph: p.paragraph,
		shaping: frozenShaping{p.arena.boxes, p.arena.glyphs, p.arena.clusters, p.arena.contexts},
		lines:   make([]frozenLine, len(p.lines))}
	for i, line := range p.lines {
		for j, knot := range line.content.knots {
			core := knot.core
			if core.Kind != KTGlue && (core.MinW != core.W || core.MaxW != core.W) {
				return nil, fmt.Errorf("freeze line %d knot %d: %w (kind %d)", i+1, j, errUnsupportedAdjustment, core.Kind)
			}
		}
		plan, err := line.planWidths()
		if err != nil {
			return nil, fmt.Errorf("freeze line %d: %w", i+1, err)
		}
		start := len(result.w)
		for j, knot := range line.content.knots {
			result.w = append(result.w, plan.content[j])
			result.pos = append(result.pos, knot.core.Pos)
			result.len = append(result.len, knot.core.Len)
			result.kind = append(result.kind, knot.core.Kind)
			result.flags = append(result.flags, knot.core.Flags)
			result.origins = append(result.origins, frozenOrigin{knot.source, knot.box, knot.origin,
				knot.variant, knot.fragment, knot.paragraphFill})
		}
		result.lines[i] = frozenLine{content: arenaRange{start, len(result.w)},
			leftSkip: plan.leftSkip, rightSkip: plan.rightSkip, width: line.targetWidth,
			byteStart: line.content.byteStart, byteEnd: line.content.byteEnd}
	}
	return result, nil
}
