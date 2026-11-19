package khipu

import (
	"fmt"
	"github.com/npillmayer/khipu/contracts"
)

// stagedDiscretionary permits a complete provider batch to be validated before
// publishing any alternative. The private arenas own all mutable storage.
type stagedDiscretionary struct {
	candidate DiscretionaryCandidate
	pre, post shapingArena
}

// prepareDiscretionaryPublication validates one alternative and owns its
// replacement storage without changing the khipu. Existing variants cannot be
// replaced: selections and arena IDs remain stable. The caller must stage the
// complete batch before committing; staging and commit require exclusive access.
func (k *FixtureKhipu) prepareDiscretionaryPublication(at int, variant uint16, penalty Penalty, pair *shapedDiscretionaryInput) (stagedDiscretionary, error) {
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

func (k *FixtureKhipu) commitDiscretionary(at int, staged stagedDiscretionary) DiscretionaryCandidate {
	// No fallible work remains. Appending preserves existing IDs and records;
	// only the new local ranges need relocation into the destination arena.
	candidate := staged.candidate
	candidate.preBox = k.shaping.appendReplacement(staged.pre)
	candidate.postBox = k.shaping.appendReplacement(staged.post)
	k.AddDiscretionaryCandidate(at, candidate)
	return candidate
}
