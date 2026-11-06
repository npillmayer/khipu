package khipu

import (
	"fmt"
	"slices"
	"strings"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/dimen"
)

const DefaultInitialKhipuCapa = 20
const MaxInitialKhipuCapa = 1024

type KnotFlags uint8

const (
	KFDiscardable KnotFlags = 1 << iota
)

type KnotCore struct {
	Pos           uint64
	W, MinW, MaxW dimen.DU
	Len           uint16
	Penalty       Penalty
	Kind          KnotType
	Flags         KnotFlags
}

func (k KnotCore) String() string {
	switch k.Kind {
	case KTGlue:
		return fmt.Sprintf("\u25cb%.2f\u00b1[p=%d]", k.W.Points(), k.Penalty)
	case KTTextBox:
		return fmt.Sprintf("\u00ab%s\u00bb(%.2f)[p=%d]", "text", k.W.Points(), k.Penalty)
	}
	return fmt.Sprintf("%d:%.2f", k.Kind, k.W.Points())
}

// Khipu stores an elastic paragraph in aligned SOA planes. Source knots carry
// original UTF-8 byte spans; a generated zero-length trailer terminates the
// paragraph. KnotByIndex returns a value snapshot, not a mutable reference.
//
// Encoded paragraphs belong to the Khipukamayuq that created them. Keep source
// spans, base knots, and shaping context stable while discretionary caches are
// in use. A breaker run replaces SelectedDiscretionaries; copy those decisions
// before rerunning if they must be retained. Concurrent mutation is unsupported.
type Khipu struct {
	owner     *Khipukamayuq
	paragraph *styled.Paragraph
	W         []dimen.DU
	MinW      []dimen.DU
	MaxW      []dimen.DU
	Penalty   []Penalty
	Pos       []uint64
	Len       []uint16
	Kind      []KnotType
	Flags     []KnotFlags
	// boxRefs is aligned with knots created through appendKnot. Zero means no
	// retained shaping output (including manually constructed knots).
	boxRefs []shapedBoxID
	shaping shapingArena

	DiscretionaryCandidates map[int][]DiscretionaryCandidate
	SelectedDiscretionaries map[int]DiscretionarySelection
}

func (kq *Khipukamayuq) allocKhipu(capacity int) *Khipu {
	assert(kq != nil, "nil-Khipukamayuq cannot allocate a khipu")
	if capacity == 0 || capacity > MaxInitialKhipuCapa {
		capacity = DefaultInitialKhipuCapa
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
	return nil
}

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

func (khipu *Khipu) isFull() bool {
	return len(khipu.Kind) == cap(khipu.Kind)
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

func (khipu *Khipu) Discardable(index int) (int, bool) {
	if index < 0 || index >= len(khipu.Kind) {
		return 0, false
	}
	discarded := false
	for i := index; i >= 0; i-- {
		if khipu.flagsAt(i)&KFDiscardable == 0 {
			return i, discarded
		}
		discarded = true
	}
	return 0, discarded
}

// acquire is called by Khipukamayuq to take ownership of a khipu.
func (kq *Khipukamayuq) acquire(khipu *Khipu) {
	assert(kq != nil, "nil-Khipukamayuq cannot acquire khipu")
	assert(khipu != nil, "nil-Khipu cannot be acquired")
	if khipu.owner == nil {
		khipu.owner = kq
	}
	assert(khipu.owner == kq, "Another Khipukamayuq already owns this Khipu")
}

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

type Khipukamayuq struct {
	env typEnv
}

// NewKhipukamayuq creates an owner for SOA paragraphs using config.
// It copies parameters and retains collaborator instances as described by Config.
// A Khipukamayuq must not be used concurrently.
func NewKhipukamayuq(config Config) *Khipukamayuq {
	env := typEnv{
		hyphenator:  config.Hyphenator,
		parFillSkip: KnotCore{MaxW: dimen.Fill},
	}
	if config.Streaming != nil {
		streaming := *config.Streaming
		streaming.Defaults.Features = slices.Clone(streaming.Defaults.Features)
		env.streaming = &streaming
	} else {
		env.streaming = defaultStreamingConfig()
	}
	if config.ParFillSkip != nil {
		env.parFillSkip = *config.ParFillSkip
	}
	if config.Params == nil {
		env.params = newParameters()
	} else {
		params := *config.Params
		env.params = &params
	}
	return &Khipukamayuq{
		env: env,
	}
}
