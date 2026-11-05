package khipu

import (
	"fmt"
	"unicode/utf8"

	"github.com/npillmayer/khipu/contracts"
)

// discretionaryInput separates original provenance from replacement input.
// source/split use original paragraph coordinates; fragment glyph cluster IDs
// will instead refer to each replacement string's local scalar coordinates.
// It deliberately carries no inferred per-character replacement-to-source map.
type discretionaryInput struct {
	source    runSourceRange
	split     SourceSplit
	pre, post replacementInput
}

// replacementInput owns the immutable replacement text (strings are values).
// provenance partitions the original request, not the replacement bytes. A
// replacement may insert/delete/change characters, or be empty. Separate later
// shaping requests must supply the appropriate pre/post contexts; this record
// does not assume that a manual cross-style break has one context on both sides.
type replacementInput struct {
	text        string
	scalarCount uint64
	provenance  runSourceRange
}

// prepareDiscretionaryInput validates source-split metadata without shaping,
// discovering opportunities, allocating arena records or mutating candidates.
// original is the same input supplied to the hyphenator; source locates it in
// the original paragraph. Both strings and ranges must describe unnormalized
// original text. Returned replacement strings do not alias mutable buffers.
//
// An explicit marker convention (for example, split after U+00AD while omitting
// it from replacement text) can use this contract, but marker discovery and
// language-specific fragment validity are not implemented here. Empty fragments
// and endpoint splits are valid provenance; discovery policy may restrict them.
// Feature ranges cannot be translated using this split alone: nontrivial
// replacement needs a separate mapping or explicit feature-resolution policy.
func prepareDiscretionaryInput(original string, source runSourceRange, fragment HyphenationFragment) (discretionaryInput, error) {
	bad := func(reason string) (discretionaryInput, error) {
		return discretionaryInput{}, fmt.Errorf("%w: discretionary provenance: %s", contracts.ErrIllegalArguments, reason)
	}
	if !utf8.ValidString(original) || !utf8.ValidString(fragment.PreBreak) || !utf8.ValidString(fragment.PostBreak) {
		return bad("invalid UTF-8")
	}
	if source.byteStart > source.byteEnd || source.scalarStart > source.scalarEnd ||
		source.byteEnd-source.byteStart != uint64(len(original)) || source.scalarEnd-source.scalarStart != uint64(utf8.RuneCountInString(original)) {
		return bad("source range does not describe original text")
	}
	if !fragment.SourceSplit.Valid {
		return bad("original split position is unknown")
	}
	offset := fragment.SourceSplit.ByteOffset
	if offset > uint64(len(original)) {
		return bad("split is outside original text")
	}
	if offset < uint64(len(original)) && !utf8.RuneStart(original[offset]) {
		return bad("split is inside UTF-8 encoding")
	}
	// Additions are bounded by the already validated original endpoints.
	byteSplit := source.byteStart + offset
	scalarSplit := source.scalarStart + uint64(utf8.RuneCountInString(original[:offset]))
	return discretionaryInput{
		source: source, split: SourceSplit{ByteOffset: byteSplit, Valid: true},
		pre: replacementInput{text: fragment.PreBreak, scalarCount: uint64(utf8.RuneCountInString(fragment.PreBreak)),
			provenance: runSourceRange{source.byteStart, byteSplit, source.scalarStart, scalarSplit}},
		post: replacementInput{text: fragment.PostBreak, scalarCount: uint64(utf8.RuneCountInString(fragment.PostBreak)),
			provenance: runSourceRange{byteSplit, source.byteEnd, scalarSplit, source.scalarEnd}},
	}, nil
}
