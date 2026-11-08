package khipu

import (
	"fmt"

	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/uax/bidi"
	"golang.org/x/text/language"
)

// LineShape supplies inclusive target widths (both physical side skips count).
// Line numbers are one-based. Implementations must remain stable during breaking
// and freezing; copying Params does not clone the service behind this interface.
type LineShape interface {
	LineLength(int32) dimen.DU
}

// Params holds paragraph-wide settings. Resolve geometry once before breaking
// and pass the same value to freezing. Zero skips are explicit, not missing
// defaults. Encoding alone does not require a LineShape.
type Params struct {
	LineShape LineShape
	// Physical edge glue, also in RTL paragraphs. Only W/MinW/MaxW are used;
	// Kind, flags and penalty do not participate in side-skip accounting.
	LeftSkip, RightSkip KnotCore
	Language            language.Tag
	Script              language.Script
	BidiDir             bidi.Direction
	// Hyphenpenalty is the fallback for a discretionary whose provider supplies
	// no nonzero penalty. The breaker reads the resulting PreBreak.Penalty;
	// it has no independent hyphen-penalty default. Zero is a valid fallback.
	Hyphenpenalty int
	// SpaceStretchPercent and SpaceShrinkPercent apply only to U+0020 in
	// elastic-spacing runs. Zero explicitly disables the corresponding change.
	// Values must be finite and nonnegative; shrink must not exceed 100.
	// Candidate-width calculation applies these percentages to shaped advances.
	SpaceStretchPercent float64
	SpaceShrinkPercent  float64
}

// DefaultParams returns independent paragraph defaults, with zero side skips
// and no line shape. Breakers offer DefaultLayout constructors for their layout
// profiles; apply caller overrides afterward, including explicit zero values.
func DefaultParams() Params { return *newParameters() }

// ValidateGeometry checks the shared layout contract without querying the
// shape or filling defaults. Targets are supplied by the shape during layout.
// Side skips are glue by role; only their width triples are interpreted.
func (p Params) ValidateGeometry() error {
	if nilRunService(p.LineShape) {
		return fmt.Errorf("%w: missing line shape", ErrIllegalArguments)
	}
	if !validGeometryWidths(p.LeftSkip) || !validGeometryWidths(p.RightSkip) {
		return fmt.Errorf("%w: invalid side-skip widths", ErrIllegalArguments)
	}
	return nil
}

func newParameters() *Params {
	var params Params
	params.Language = language.English
	params.Script = language.MustParseScript("Latn")
	params.BidiDir = bidi.LeftToRight
	params.Hyphenpenalty = 10
	params.SpaceStretchPercent = 50
	params.SpaceShrinkPercent = 33.3
	return &params
}
