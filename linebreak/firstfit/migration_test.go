package firstfit

import (
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/internal/search"
	"github.com/npillmayer/khipu/internal/testsupport"
)

// breakFixture exercises the same private algorithm against mutable metric
// fixtures. Public immutable publication has separate integration coverage.
func breakFixture(m search.Paragraph, layout khipu.Params, p *Parameters) ([]int, error) {
	return breakParagraph(m, layout, p, nil)
}

type fixtureProvider struct{}

func (fixtureProvider) DiscretionaryCandidates(m search.Paragraph, i int) ([]khipu.DiscretionaryCandidate, error) {
	return (search.Provider{Discovery: m.(*testsupport.Paragraph).Discovery}).DiscretionaryCandidates(m, i)
}
