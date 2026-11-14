package khipu

import (
	"context"
	"fmt"
	"io"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/uax/bidi"
	textbidi "golang.org/x/text/unicode/bidi"
)

// HasExplicitDiscretionary reports retained U+00AD source metadata inside a
// streaming textbox. It does not call a policy, shape fragments, or promise an
// eligible variant. Breakers use it to request explicit opportunities without
// eagerly querying automatic hyphenation for every textbox.
func (k *Khipu) HasExplicitDiscretionary(at int) bool {
	if k == nil || at < 0 || at >= len(k.Kind) || k.Kind[at] != KTTextBox || at >= len(k.shaping.sources) {
		return false
	}
	if k.hyphenationPolicy && k.hyphenation == NoHyphenation {
		return false
	}
	s := k.shaping.sources[at].source
	if s.scalarEnd > uint64(len(k.shaping.boundaries)) {
		return false
	}
	for p := s.scalarStart; p < s.scalarEnd; p++ {
		if k.shaping.boundaries[p].softHyphen {
			return true
		}
	}
	return false
}

// streamingDiscretionaryCandidates chooses explicit marker preparation or
// eligible whole-segment automatic discovery, never metric shaping. Explicit
// requests have precedence and never fall back to automatic discovery.
// All alternatives are shaped and staged before committing the complete batch;
// errors leave arena/cache unchanged, so retry cannot see a partial batch.
// The provider interface has no context: each stream uses Background,
// with internal cancellation/joining on failure. Caller cancellation requires a
// future provider-interface extension.
func (kq *Khipukamayuq) streamingDiscretionaryCandidates(k *Khipu, at int) ([]DiscretionaryCandidate, error) {
	if k.prepared != nil {
		return kq.importExplicitBatch(k, at)
	}
	explicit := k.HasExplicitDiscretionary(at)
	if !explicit && k.hyphenationPolicy && k.hyphenation != AutomaticHyphenation {
		return nil, nil
	}
	if !explicit && !k.automaticHyphenationEligible(at) {
		return nil, nil
	}
	if at >= len(k.boxRefs) || k.boxRefs[at] <= 0 || int(k.boxRefs[at]) > len(k.shaping.boxes) {
		return nil, ErrIllegalArguments
	}
	box := k.shaping.boxes[k.boxRefs[at]-1]
	if box.replacement != nil || box.context < 0 || box.context >= len(k.shaping.contexts) {
		return nil, ErrIllegalArguments
	}
	run := k.shaping.contexts[box.context]
	s := box.source
	if s.scalarStart < run.source.scalarStart || s.scalarEnd > run.source.scalarEnd {
		return nil, ErrIllegalArguments
	}
	// Read only the requested box, not a flattened copy of the paragraph.
	raw, err := io.ReadAll(k.paragraph.Raw().BoundedReader(s.byteStart, s.byteEnd))
	if err != nil {
		return nil, err
	}
	var inputs []preparedDiscretionary
	if explicit {
		inputs, err = kq.prepareExplicitDiscretionaries(string(raw), s)
	} else {
		policyParams := *kq.env.params
		policyParams.Language, policyParams.Script = run.params.shaping.Language, run.params.shaping.Script
		policyParams.BidiDir = bidi.LeftToRight
		if run.params.shaping.Direction == textbidi.RightToLeft {
			policyParams.BidiDir = bidi.RightToLeft
		}
		inputs, err = kq.prepareAutomaticDiscretionaries(string(raw), s, policyParams)
	}
	if err != nil || len(inputs) == 0 {
		return nil, err
	}
	cfg := kq.env.streaming
	if cfg == nil || cfg.NewShaper == nil {
		return nil, ErrStreamingDiscretionariesUnsupported
	}
	params, err := replacementParameters(run, s)
	if err != nil {
		return nil, err
	}
	batch := make([]stagedDiscretionary, 0, len(inputs))
	for _, input := range inputs {
		pair, err := shapeDiscretionaryInput(context.Background(), input.input, params, params, cfg.NewShaper, cfg.BufferOptions)
		if err != nil {
			return nil, err
		}
		staged, err := k.prepareDiscretionaryPublication(at, input.variant, input.penalty, pair)
		if err != nil {
			return nil, err
		}
		batch = append(batch, staged)
	}
	for _, staged := range batch {
		k.commitDiscretionary(at, staged)
	}
	return k.DiscretionariesAt(at), nil
}

func replacementParameters(run preparedRun, s runSourceRange) (runParameters, error) {
	params := run.params
	features, err := intersectFeatures(params.shaping.Features, run.source.scalarEnd-run.source.scalarStart,
		s.scalarStart-run.source.scalarStart, s.scalarEnd-run.source.scalarStart)
	if err != nil {
		return runParameters{}, err
	}
	for i := range features {
		if features[i].Start != 0 || uint64(features[i].End) != s.scalarEnd-s.scalarStart {
			return runParameters{}, fmt.Errorf("%w: replacement has range-limited features", contracts.ErrIllegalArguments)
		}
		// A feature covering the entire original box applies to both complete
		// fragments. Interior feature boundaries cannot be guessed after edits.
		features[i].Start, features[i].End = 0, 0
	}
	params.shaping.Features = features
	return params, nil
}
