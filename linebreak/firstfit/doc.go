// Package firstfit breaks immutable paragraph bindings into independent Tikrays.
// BreakParagraph accepts khipu.BreakRequest and typed Parameters; nil parameters
// select resolved algorithm defaults. Geometry is explicit and captured once per
// queried line. Discovery supplies immutable replacement opportunities. Search
// state and selections stay private to each invocation; errors return nil plans.
// Freeze the result with khipu.Freeze, without another geometry callback.
// ContinueParagraph accepts a certified NoHyphenation checkpoint, preserving
// accepted decisions/widths and searching only the suffix. A KP prefix is valid.
// Public typesetting calls are synchronous and have no context argument.
package firstfit
