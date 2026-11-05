package contracts

import (
	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/dimen"
)

type RuneReader interface {
	ReadRune() (rune, int, error)
}

// Shaper supplies metrics in logical source order. SetSource starts a new run
// and resets its byte offsets. A metric's clusterPos is the run-relative UTF-8
// byte offset of its cluster's first rune, not a sequential cluster number.
// Repeated offsets accumulate width; offsets must not decrease. Zero-width
// metrics still mark cluster starts. EOF carries no metric and all other errors
// abort encoding. sourceLen is informational; cluster positions define spans.
type Shaper interface {
	SetSource(RuneReader)
	SetStyle(styled.BidiStyleChange)
	ReadClusterMetrics() (clusterPos int, width dimen.DU, sourceLen int, err error)
}
