package khipu

import (
	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/dimen"
	"golang.org/x/text/language"
)

// StyleResolver translates an application's opaque style into concrete
// parameters for one already identified run. For example, a bold style selects
// a heavier FontFace while inheriting unchanged properties from defaults.
// Implementations know the concrete styled.Style representation and may use a
// FontLoader or a font cache internally.
//
// ResolveStyle returns a complete configuration, not a property delta. Nil style
// means no overrides of defaults. An implementation must report an error if it
// cannot interpret a style or resolve a required property; successful results
// must have a non-nil Font and positive EmSize. Fixed spacing and an empty
// feature list are valid explicit values, not missing-value sentinels.
//
// Neither style nor defaults (including its Features slice) may be mutated.
// Returned feature storage must remain stable; run preparation will copy it
// before publication. The caller supplies inherited defaults independently for
// each run rather than inheriting from the preceding run's result.
//
// This interface does not inspect or subdivide text for script changes or font
// fallback. Script and language are resolved from defaults and explicit style
// information. Bidi analysis remains separate. Khipukamayuq obtains FontScale
// from the resolved Font and EmSize after this operation succeeds.
// The interface is a draft seam; the current encoder does not call it yet.
type StyleResolver interface {
	ResolveStyle(style styled.Style, defaults ResolvedRunStyle) (ResolvedRunStyle, error)
}

// ResolvedRunStyle is the style-controlled portion of a run's parameters.
// Features use ShapeFeature's run-relative scalar coordinates. Direction and
// bidi level come from paragraph analysis, while scale is derived from Font
// and EmSize; none of those is resolved through the style interface.
type ResolvedRunStyle struct {
	Font     FontFace
	EmSize   dimen.DU
	Script   language.Script
	Language language.Tag
	Features []ShapeFeature
	Spacing  SpacingMode
}

// SpacingMode describes interword-spacing policy. It neither determines
// discardability nor prescribes numerical stretch/shrink capacities.
type SpacingMode uint8

const (
	SpacingFixed   SpacingMode = iota // Interword spaces retain their natural width.
	SpacingElastic                    // Interword spaces may receive stretch/shrink capacity.
)
