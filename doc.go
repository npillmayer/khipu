// Package khipu prepares styled text as immutable struct-of-arrays material,
// binds atomic advances, and freezes independent linebreaking trials.
//
// Khipukamayuq.PrepareParagraph returns a Khipu and its synchronized Discovery.
// EncodeParagraph is the text-only convenience. Config{} selects deterministic
// synthetic streaming shaping; applications supply Config.Streaming for real
// fonts/styles and an optional Hyphenator. Original UTF-8 coordinates, whole-
// paragraph bidi context and scalar feature domains survive preparation.
//
// BindParagraph overlays copied signed atomic advances without copying base
// knots or shaping. The firstfit and knuthplass packages accept a BreakRequest
// and return an immutable Tikray: complete endpoints, selected replacements,
// captured geometry, effective policies and diagnostics. Later trials never
// replace earlier decisions. Freeze consumes only the chosen Tikray and performs
// no shape callback, discovery or shaping. FrozenKhipu retains stable glyph/source
// data independently of preparation, bindings, discovery and trial workspaces.
//
// MeasureIntrinsic supports ordinary and bounded explicit hyphenation without
// target geometry. Checkpoint certifies an accepted prefix of a complete Tikray;
// firstfit.ContinueParagraph searches only its NoHyphenation suffix. Prefix
// widths/decisions are retained, while changed suffix atomic advances are allowed.
// Drop material, discovery, plans and checkpoints when each paragraph has fully
// flowed; frozen fragments remain usable. Vertical assembly/rendering are outside
// this module. No public typesetting operation takes a context. Internal streaming
// failures still cancel, drain and join producers before cleanup.
//
// Source/styles/font services remain stable under their borrowing contracts.
// Discovery serializes cache misses; shared collaborators across incarnations
// still need their own serialization. Independent trials own their search state.
// Package contracts defines backend interfaces without concrete font types;
// helpers supplies synthetic defaults, not production font shaping.
package khipu
