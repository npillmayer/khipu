package khipu

import "slices"

// ResumePosition identifies the next original knot and optional pending post-break
// material. Carry identity is meaningful even for an empty/zero-width replacement.
type ResumePosition struct {
	NextKnot int
	Carry    AlternativeID
}

// BreakCheckpoint certifies a copied accepted prefix from a complete Tikray. It
// retains the incarnation, not a chain of plans/checkpoints or a discovery cache.
type BreakCheckpoint struct {
	material      *Khipu
	prefix        []LineDecision
	replacements  map[AlternativeID]*alternative
	policies      []PolicyRecord
	linePolicy    []int
	prefixAtomics []AtomicAdvance
	resume        ResumePosition
}

// ContinuationSeed is detached breaker scratch, never an independent certificate.
// Editing it cannot change the checkpoint or authorize a changed locked prefix.
type ContinuationSeed struct {
	Prefix    []LineDecision
	Resume    ResumePosition
	PostBreak KnotCore
}

// AcceptedLines reports the number of complete locked lines.
func (s ContinuationSeed) AcceptedLines() int { return len(s.Prefix) }

// Checkpoint certifies zero through all accepted lines. Initially only
// NoHyphenation material is supported; general replacement carry stays deferred.
func Checkpoint(p *Tikray, accepted int) (*BreakCheckpoint, error) {
	if p == nil || p.Incarnation().token == nil || accepted < 0 || accepted > len(p.lines) {
		return nil, ErrIllegalArguments
	}
	if p.binding.material.hyphenation != NoHyphenation {
		return nil, ErrUnsupportedPolicy
	}
	c := &BreakCheckpoint{material: p.binding.material, prefix: slices.Clone(p.lines[:accepted]), replacements: make(map[AlternativeID]*alternative), policies: slices.Clone(p.policies), linePolicy: slices.Clone(p.linePolicy[:accepted])}
	if accepted > 0 {
		end := c.prefix[accepted-1].End
		c.resume.NextKnot = end.Knot + 1
		if end.Kind == DiscretionaryBoundary {
			c.resume.Carry = end.Alternative
		}
	}
	for _, line := range c.prefix {
		if line.End.Kind == DiscretionaryBoundary {
			id := line.End.Alternative
			c.replacements[id] = p.replacements[id]
		}
	}
	for i := 0; i < c.resume.NextKnot; i++ {
		t, _ := c.material.Knot(i)
		if t.Atomic != 0 {
			c.prefixAtomics = append(c.prefixAtomics, AtomicAdvance{ID: t.Atomic, Advance: p.binding.advances[c.material.atomicSlots[t.Atomic]]})
		}
	}
	return c, nil
}
func (c *BreakCheckpoint) validate(b *AtomicBinding) error {
	if c == nil || c.material == nil || c.material.ID().token == nil || b == nil || b.Incarnation() != c.material.ID() {
		return ErrIllegalArguments
	}
	if c.material.hyphenation != NoHyphenation {
		return ErrUnsupportedPolicy
	}
	for _, a := range c.prefixAtomics {
		if b.advances[b.material.atomicSlots[a.ID]] != a.Advance {
			return ErrIllegalArguments
		}
	}
	return nil
}

// SeedFor checks identity and consumed atomic advances without prefix scanning,
// width queries or services. Errors return a zero seed. Suffix-only advances may change.
func (c *BreakCheckpoint) SeedFor(b *AtomicBinding) (ContinuationSeed, error) {
	if e := c.validate(b); e != nil {
		return ContinuationSeed{}, e
	}
	s := ContinuationSeed{Prefix: slices.Clone(c.prefix), Resume: c.resume}
	if s.Resume.Carry != (AlternativeID{}) {
		s.PostBreak = c.replacements[s.Resume.Carry].postCore
	}
	return s, nil
}
