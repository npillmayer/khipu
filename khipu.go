package khipu

import (
	"strings"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/dimen"
)

// DefaultInitialKhipuCapa is the initial knot capacity when allocation has no
// size estimate. Paragraph storage grows as needed; this is not a size limit.
//
// Deprecated: allocation capacity is an implementation detail. Use owner
// encoding or preparation operations without depending on a specific capacity.
const DefaultInitialKhipuCapa = defaultInitialKhipuCapa

const defaultInitialKhipuCapa = 20

// Khipu stores an elastic paragraph in aligned SOA planes. Source knots carry
// original UTF-8 byte spans; a generated zero-length trailer terminates the
// paragraph. KnotByIndex returns a value snapshot, not a mutable reference.
//
// Encoded paragraphs belong to the Khipukamayuq that created them. Keep source
// spans, base knots, and shaping context stable while discretionary caches are
// in use. A successful breaker run replaces SelectedDiscretionaries; copy those
// decisions before rerunning if needed. Concurrent mutation is unsupported.
//
// All exported planes describe the same knot indices and must remain aligned.
// A shallow copy shares their storage and the candidate/selection maps. Treat
// encoded base planes as read-only: direct edits can invalidate private shaping
// provenance. Manually constructed metric paragraphs can be used by breakers,
// but lack the ownership/provenance required by [Khipukamayuq.Freeze] and
// [Khipu.BreakpointPositions]. Prefer owner operations for normal client use.
type Khipu struct {
	owner         *Khipukamayuq
	paragraph     *styled.Paragraph
	prepared      *PreparedParagraph // Explicit-only lazy cache association; never frozen.
	boundMaterial *preparedMaterial  // Preparation identity for checkpoints; never frozen.
	// W, MinW and MaxW contain natural, minimum and maximum widths in DU.
	W    []dimen.DU
	MinW []dimen.DU
	MaxW []dimen.DU
	// Penalty contains the break penalty after each knot.
	Penalty []Penalty
	// Pos and Len contain absolute UTF-8 byte starts and original byte lengths.
	Pos []uint64
	Len []uint16
	// Kind identifies material; Flags controls trimming independently.
	Kind  []KnotType
	Flags []KnotFlags
	// boxRefs is aligned with knots created through appendKnot. Zero means no
	// retained shaping output (including manually constructed knots).
	boxRefs []shapedBoxID
	shaping shapingArena
	// Legacy EncodeParagraph leaves this gate unset for compatibility.
	hyphenationPolicy bool
	hyphenation       HyphenationMode

	// DiscretionaryCandidates caches alternatives by source textbox index.
	// Variants must remain unique and stable within each textbox. Missing entries
	// do not distinguish unqueried textboxes from successful empty discoveries.
	DiscretionaryCandidates map[int][]DiscretionaryCandidate
	// SelectedDiscretionaries records choices by line-ending knot index. Successful
	// breaker calls replace this map's contents; failed calls preserve them.
	// Keep these choices with their matching breakpoint list until freezing.
	SelectedDiscretionaries map[int]DiscretionarySelection
	// discretionaryEmpty records successful empty discovery by this paragraph's
	// owner. It is separate from the public candidate map: absence there still
	// means no alternatives, not necessarily that discovery has been attempted.
	// Source, shaping context and collaborator policy must stay stable; re-encode
	// with a new owner to use a changed policy. Errors never populate this set.
	discretionaryEmpty map[int]struct{}
}

// allocKhipu assigns ownership at construction and reserves the known knot
// count. Zero selects the small default; larger paragraphs are not capped back
// to that default. Slice append handles subsequent growth without an isFull test.
func (kq *Khipukamayuq) allocKhipu(capacity int) *Khipu {
	assert(kq != nil, "nil-Khipukamayuq cannot allocate a khipu")
	if capacity == 0 {
		capacity = defaultInitialKhipuCapa
	}
	khipu := Khipu{
		owner:   kq,
		W:       make([]dimen.DU, 0, capacity),
		MinW:    make([]dimen.DU, 0, capacity),
		MaxW:    make([]dimen.DU, 0, capacity),
		Penalty: make([]Penalty, 0, capacity),
		Pos:     make([]uint64, 0, capacity),
		Len:     make([]uint16, 0, capacity),
		Kind:    make([]KnotType, 0, capacity),
		Flags:   make([]KnotFlags, 0, capacity),

		DiscretionaryCandidates: make(map[int][]DiscretionaryCandidate),
		SelectedDiscretionaries: make(map[int]DiscretionarySelection),
	}
	return &khipu
}

func unwireKhipu(khipu *Khipu) *Khipu {
	khipu.owner = nil
	khipu.paragraph = nil
	khipu.prepared = nil
	khipu.boundMaterial = nil
	khipu.W = nil
	khipu.MinW = nil
	khipu.MaxW = nil
	khipu.Penalty = nil
	khipu.Pos = nil
	khipu.Len = nil
	khipu.Kind = nil
	khipu.Flags = nil
	khipu.boxRefs = nil
	khipu.shaping = shapingArena{}
	khipu.DiscretionaryCandidates = nil
	khipu.SelectedDiscretionaries = nil
	khipu.discretionaryEmpty = nil
	return nil
}

// KnotByIndex returns a detached snapshot at the zero-based index, or a zero
// KnotCore if index is outside the Kind plane. It does not validate alignment:
// short metric/source planes may panic. Missing flag entries are read as zero.
func (khipu Khipu) KnotByIndex(index int) KnotCore {
	if index < 0 || index >= len(khipu.Kind) {
		return KnotCore{}
	}
	return KnotCore{
		W:       khipu.W[index],
		MinW:    khipu.MinW[index],
		MaxW:    khipu.MaxW[index],
		Kind:    khipu.Kind[index],
		Penalty: khipu.Penalty[index],
		Pos:     khipu.Pos[index],
		Len:     khipu.Len[index],
		Flags:   khipu.flagsAt(index),
	}
}

func (khipu *Khipu) appendKnot(k KnotCore) *Khipu {
	// Pad references for manually constructed metric-only SOA values.
	for len(khipu.boxRefs) < len(khipu.Kind) {
		khipu.boxRefs = append(khipu.boxRefs, 0)
	}
	khipu.boxRefs = append(khipu.boxRefs, 0)
	if khipu.shaping.sources != nil {
		khipu.shaping.sources = append(khipu.shaping.sources, knotSource{})
	}
	khipu.W = append(khipu.W, k.W)
	khipu.MinW = append(khipu.MinW, k.MinW)
	khipu.MaxW = append(khipu.MaxW, k.MaxW)
	khipu.Penalty = append(khipu.Penalty, k.Penalty)
	khipu.Pos = append(khipu.Pos, k.Pos)
	khipu.Len = append(khipu.Len, k.Len)
	khipu.Kind = append(khipu.Kind, k.Kind)
	khipu.Flags = append(khipu.Flags, k.Flags)
	return khipu
}

// Unskip removes the last knot if it is discardable glue or kern.
// A non-discardable paragraph trailer is retained.
func (khipu *Khipu) Unskip() {
	if len(khipu.W) == 0 {
		return
	}
	top := len(khipu.W) - 1
	if (khipu.Kind[top] == KTGlue || khipu.Kind[top] == KTKern) && khipu.flagsAt(top)&KFDiscardable != 0 {
		khipu.W = khipu.W[:top]
		khipu.MinW = khipu.MinW[:top]
		khipu.MaxW = khipu.MaxW[:top]
		khipu.Penalty = khipu.Penalty[:top]
		khipu.Pos = khipu.Pos[:top]
		khipu.Len = khipu.Len[:top]
		khipu.Kind = khipu.Kind[:top]
		if len(khipu.Flags) > top {
			khipu.Flags = khipu.Flags[:top]
		}
		if len(khipu.boxRefs) > top {
			khipu.boxRefs = khipu.boxRefs[:top]
		}
		if len(khipu.shaping.sources) > top {
			khipu.shaping.sources = khipu.shaping.sources[:top]
		}
	}
}

// String joins the knots' diagnostic descriptions in logical storage order.
func (khipu *Khipu) String() string {
	var sb strings.Builder
	for i := range len(khipu.Kind) {
		if i > 0 {
			sb.WriteString(" ")
		}
		sb.WriteString(khipu.KnotByIndex(i).String())
	}
	return sb.String()
}

func assert(cond bool, msg string) {
	if !cond {
		panic(msg)
	}
}
