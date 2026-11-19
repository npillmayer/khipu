package knuthplass

import (
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/internal/search"
	"github.com/npillmayer/khipu/internal/testsupport"
)

// breakFixture exercises the same private algorithm against mutable metric
// fixtures. Public immutable publication has separate integration coverage.
func breakFixture(m search.Paragraph, layout khipu.Params, p *Parameters) ([]int, error) {
	r, e := selectParagraphBreaks(m, layout, p)
	if e != nil {
		return nil, e
	}
	recordSelectedDiscretionaries(m, r.breaks)
	return projectBreakRefs(r.breaks), nil
}

type fixtureProvider struct{}

func (fixtureProvider) DiscretionaryCandidates(m search.Paragraph, i int) ([]khipu.DiscretionaryCandidate, error) {
	return (search.Provider{Discovery: m.(*testsupport.Paragraph).Discovery}).DiscretionaryCandidates(m, i)
}
