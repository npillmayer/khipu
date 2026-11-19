package khipu

import (
	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/dimen"
)

// freezeInput exists only within Freeze. Its mutable substitution tables never
// escape into material, discovery, plans or frozen results.
type freezeInput struct {
	paragraph *styled.Paragraph
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
	boxRefs                 []shapedBoxID
	shaping                 shapingArena
	DiscretionaryCandidates map[int][]DiscretionaryCandidate
	SelectedDiscretionaries map[int]DiscretionarySelection
}

func (khipu freezeInput) KnotByIndex(index int) KnotCore {
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
		Flags:   khipu.Flags[index],
	}
}

func (k *freezeInput) DiscretionariesAt(i int) []DiscretionaryCandidate {
	return k.DiscretionaryCandidates[i]
}
func (k *freezeInput) SelectedDiscretionaryAt(i int) (DiscretionarySelection, bool) {
	v, ok := k.SelectedDiscretionaries[i]
	return v, ok
}
func (k *freezeInput) flagsAt(i int) KnotFlags {
	if i < 0 || i >= len(k.Flags) {
		return 0
	}
	return k.Flags[i]
}
