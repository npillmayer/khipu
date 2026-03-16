package khipu

// DiscretionaryCandidate stores one lazily discovered hyphenation alternative
// associated with a textbox knot. Variants are expected to be stable for a given
// textbox within one paragraph instance.
type DiscretionaryCandidate struct {
	Variant   uint16
	PreBreak  KnotCore
	PostBreak KnotCore
}

// DiscretionarySelection records which discretionary candidate was selected
// for a rendered break decision.
type DiscretionarySelection struct {
	Source  int
	Variant uint16
}

func defaultFlagsForKind(kind KnotType) KnotFlags {
	switch kind {
	case KTGlue, KTKern:
		return KFDiscardable
	default:
		return 0
	}
}

func (khipu *Khipu) flagsAt(index int) KnotFlags {
	if khipu == nil || index < 0 || index >= len(khipu.Flags) {
		return 0
	}
	return khipu.Flags[index]
}

func (k KnotCore) IsDiscardable() bool {
	return k.Flags&KFDiscardable != 0
}

func (khipu *Khipu) SetKnotFlags(index int, flags KnotFlags) bool {
	if khipu == nil || index < 0 || index >= len(khipu.Kind) {
		return false
	}
	if len(khipu.Flags) < len(khipu.Kind) {
		missing := len(khipu.Kind) - len(khipu.Flags)
		khipu.Flags = append(khipu.Flags, make([]KnotFlags, missing)...)
	}
	khipu.Flags[index] = flags
	return true
}

func (khipu *Khipu) AddDiscretionaryCandidate(index int, cand DiscretionaryCandidate) bool {
	if khipu == nil || index < 0 || index >= len(khipu.Kind) {
		return false
	}
	if khipu.DiscretionaryCandidates == nil {
		khipu.DiscretionaryCandidates = make(map[int][]DiscretionaryCandidate)
	}
	khipu.DiscretionaryCandidates[index] = append(khipu.DiscretionaryCandidates[index], cand)
	return true
}

func (khipu *Khipu) DiscretionariesAt(index int) []DiscretionaryCandidate {
	if khipu == nil || index < 0 {
		return nil
	}
	return khipu.DiscretionaryCandidates[index]
}

func (khipu *Khipu) SelectDiscretionary(breakpoint int, choice DiscretionarySelection) bool {
	if khipu == nil || breakpoint < 0 || breakpoint >= len(khipu.Kind) {
		return false
	}
	if khipu.SelectedDiscretionaries == nil {
		khipu.SelectedDiscretionaries = make(map[int]DiscretionarySelection)
	}
	khipu.SelectedDiscretionaries[breakpoint] = choice
	return true
}

func (khipu *Khipu) SelectedDiscretionaryAt(breakpoint int) (DiscretionarySelection, bool) {
	if khipu == nil || breakpoint < 0 {
		return DiscretionarySelection{}, false
	}
	choice, ok := khipu.SelectedDiscretionaries[breakpoint]
	return choice, ok
}
