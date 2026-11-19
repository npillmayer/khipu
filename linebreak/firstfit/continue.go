package firstfit

import (
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/internal/search"
)

// BreakParagraph searches a fresh immutable binding and publishes a complete
// Tikray. It uses minimum-width first-fit with the existing local hyphenation
// rescue policy. Nil parameters select defaults; Prefix must be nil.
func BreakParagraph(r khipu.BreakRequest, params *Parameters) (*khipu.Tikray, error) {
	if r.Prefix != nil {
		return nil, khipu.ErrIllegalArguments
	}
	return trial(r, params, false)
}

// ContinueParagraph retains a certified prefix exactly and searches only the
// NoHyphenation suffix. Prefix must be nonnil. Accepted widths are never queried.
func ContinueParagraph(r khipu.BreakRequest, params *Parameters) (*khipu.Tikray, error) {
	if r.Prefix == nil {
		return nil, khipu.ErrIllegalArguments
	}
	return trial(r, params, true)
}
func trial(r khipu.BreakRequest, params *Parameters, continued bool) (*khipu.Tikray, error) {
	if e := r.Validate(); e != nil {
		return nil, e
	}
	if params == nil {
		params = NewDefaultParameters()
	}
	local := *params
	local.discretionaryProvider = search.Provider{Discovery: r.Discovery}
	var seed *breakSeed
	var prefix []khipu.LineDecision
	if continued {
		s, e := r.Prefix.SeedFor(r.Binding)
		if e != nil {
			return nil, e
		}
		prefix = s.Prefix
		seed = &breakSeed{}
		for _, l := range prefix {
			seed.prefix = append(seed.prefix, l.End.Knot)
		}
	}
	widths := search.NewWidths(r.Geometry.Shape, prefix)
	m := search.New(r.Binding, r.Discovery)
	ends, e := breakParagraph(m, widths.Layout(r.Geometry), &local, seed)
	if widths.Err != nil {
		return nil, widths.Err
	}
	if e != nil {
		return nil, e
	}
	decisions, e := widths.Decisions(r, ends, m.SelectedDiscretionaries, prefix)
	if e != nil {
		return nil, e
	}
	return khipu.NewTikray(r, decisions, khipu.PolicyRecord{Algorithm: khipu.FirstFitAlgorithm, FirstFit: khipu.FirstFitSettings{MinHyphenGain: local.MinHyphenGain}}, khipu.BreakDiagnostics{})
}

type breakSeed struct{ prefix []int }
