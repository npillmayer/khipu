package khipu

import (
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/uax/bidi"
	"golang.org/x/text/language"
)

type Params struct {
	Language        language.Tag
	Script          language.Script
	BidiDir         bidi.Direction
	Baselineskip    dimen.DU
	Lineskip        dimen.DU
	Lineskiplimit   dimen.DU
	Hypenchar       rune
	Hyphenpenalty   int
	Minhyphenlength dimen.DU
	// SpaceStretchPercent and SpaceShrinkPercent apply only to U+0020 in
	// elastic-spacing runs. Zero explicitly disables the corresponding change.
	// Values must be finite and nonnegative; shrink must not exceed 100.
	// Candidate-width calculation applies these percentages to shaped advances.
	SpaceStretchPercent float64
	SpaceShrinkPercent  float64
}

func newParameters() *Params {
	var params Params
	params.Language = language.English
	params.Script = language.MustParseScript("Latn")
	params.BidiDir = bidi.LeftToRight
	params.Baselineskip = 12 * dimen.PT
	params.Lineskip = 0
	params.Lineskiplimit = 0
	params.Hypenchar = rune('-')
	params.Hyphenpenalty = 10
	params.Minhyphenlength = 2
	params.SpaceStretchPercent = 50
	params.SpaceShrinkPercent = 33.3
	return &params
}
