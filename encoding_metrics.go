package khipu

import (
	"fmt"
	"io"
	"math"
	"unicode/utf8"

	"github.com/npillmayer/khipu/dimen"
)

type clusterMetrics struct {
	widths []dimen.DU
	starts []bool // explicit cluster boundaries, including the end of the run
}

func getMetrics(source []byte, shaper Shaper) (clusterMetrics, error) {
	m := clusterMetrics{widths: make([]dimen.DU, len(source)), starts: make([]bool, len(source)+1)}
	m.starts[0] = true
	m.starts[len(source)] = true
	previous := -1
	for {
		cluster, w, _, err := shaper.ReadClusterMetrics()
		if err == io.EOF {
			return m, nil
		} // EOF carries no metric
		if err != nil {
			return clusterMetrics{}, err
		}
		if cluster < 0 || cluster >= len(source) || cluster < previous || !utf8.RuneStart(source[cluster]) {
			return clusterMetrics{}, fmt.Errorf("khipu: invalid cluster byte offset %d", cluster)
		}
		sum := int64(m.widths[cluster]) + int64(w)
		if sum < math.MinInt32 || sum > math.MaxInt32 {
			return clusterMetrics{}, fmt.Errorf("khipu: cluster width overflow")
		}
		m.widths[cluster] = dimen.DU(sum)
		m.starts[cluster] = true
		previous = cluster
	}
}

func (m clusterMetrics) width(from, to int) (dimen.DU, error) {
	var w int64
	for _, v := range m.widths[from:to] {
		w += int64(v)
	}
	if w < math.MinInt32 || w > math.MaxInt32 {
		return 0, fmt.Errorf("khipu: knot width overflow")
	}
	return dimen.DU(w), nil
}
