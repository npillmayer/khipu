// Package khipu encodes styled text into struct-of-arrays paragraphs for
// linebreaking. Khipu stores widths, UTF-8 source spans, flags, and penalties in
// aligned planes; KnotByIndex returns a KnotCore value snapshot.
//
// Construct a Khipukamayuq with Config and call its EncodeParagraph method.
// Config{} selects a fixed-width metric provider. Applications supply a contracts.Shaper
// for font-backed metrics and an optional Hyphenator for lazy alternatives.
// Encoding preserves original source bytes and appends the final fill/forced
// break, including for empty paragraphs. Normalize text before establishing
// styled source ranges if needed.
//
// The linebreak/firstfit and linebreak/knuthplass packages consume these
// paragraphs and return line-ending knot indices. To discover hyphenation
// alternatives lazily, set the breaker's DiscretionaryProvider to the owner.
// Chosen variants are retained in Khipu.SelectedDiscretionaries; each breaker
// run replaces the previous selections.
//
// Owners and their mutable collaborators must not be used concurrently.
// Linebreaking does not produce final glyph placement or rendering: freezing
// and glyph provenance remain future work. Package contracts owns the service
// interfaces and their value types. Package helpers provides deterministic
// defaults: FixedWidthShaper for Config, and SyntheticFont, DefaultStyleResolver,
// and SyntheticStreamingShaper for standalone font/style/streaming work. The
// encoder does not consume those draft streaming services yet. Root aliases and
// forwarding constructors preserve earlier client code; new code should import
// contracts and helpers directly. Paragraph-specific interfaces stay here.
//
/*
License

Governed by a 3-Clause BSD license. License file may be found in the root
folder of this module.

Copyright © 2017–2021 Norbert Pillmayer <norbert@pillmayer.com>

*/
package khipu

import (
	"errors"

	"github.com/npillmayer/khipu/contracts"

	"github.com/npillmayer/schuko/tracing"
)

// tracer traces with key 'khipu'.
func tracer() tracing.Trace {
	return tracing.Select("khipu")
}

func assert(cond bool, msg string) {
	if !cond {
		panic(msg)
	}
}

var (
	// ErrIllegalArguments retains identity with contracts.ErrIllegalArguments.
	ErrIllegalArguments = contracts.ErrIllegalArguments
	ErrVoidKhipukamayuq = errors.New("khipu: void khipukamayuq")
)
