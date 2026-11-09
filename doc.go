// Package khipu encodes styled text into struct-of-arrays paragraphs for
// linebreaking. Khipu stores widths, UTF-8 source spans, flags, and penalties in
// aligned planes; KnotByIndex returns a KnotCore value snapshot.
//
// Construct a Khipukamayuq with Config and call EncodeParagraph.
// Config{} selects synthetic streaming shaping for unstyled text. Applications
// supply Config.Streaming for font-backed shaping and style resolution, and an
// optional Hyphenator for lazy alternatives. EncodeParagraph takes a context
// and a styled paragraph; shaping always uses the streaming contract.
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
// Linebreaking does not produce final glyph placement or rendering. Internal
// line materialization and glue-only freezing are exposed through Freeze,
// returning an opaque FrozenKhipu. Streaming paragraphs retain glyph/source provenance.
// Package contracts owns the service
// interfaces and their value types. Package helpers provides deterministic
// defaults: SyntheticFont, DefaultStyleResolver, and SyntheticStreamingShaper.
// The default resolver rejects application-defined styles; synthetic glyphs
// are not real font shaping. Import contracts and helpers directly; the root
// package does not re-export their service types or constructors.
// Paragraph-specific interfaces stay here.
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
