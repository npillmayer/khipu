package khipu

import (
	"errors"
	"io"
	"math"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/dimen"
)

type metricRecord struct {
	at  int
	w   dimen.DU
	err error
}

type recordedShaper struct{ records []metricRecord }

func (*recordedShaper) SetSource(RuneReader)            {}
func (*recordedShaper) SetStyle(styled.BidiStyleChange) {}
func (s *recordedShaper) ReadClusterMetrics() (int, dimen.DU, int, error) {
	if len(s.records) == 0 {
		return 999, 999, 0, io.EOF
	}
	r := s.records[0]
	s.records = s.records[1:]
	return r.at, r.w, 0, r.err
}

func TestClusterMetricBoundaries(t *testing.T) {
	m, err := getMetrics([]byte("é x"), &recordedShaper{[]metricRecord{{0, 2, nil}, {0, 3, nil}, {2, 0, nil}, {3, 7, nil}}})
	if err != nil {
		t.Fatal(err)
	}
	if m.widths[0] != 5 || !m.starts[2] || m.starts[1] || !m.starts[4] {
		t.Fatalf("cluster starts must be independent of width: %+v", m)
	}
	if w, err := m.width(0, 4); err != nil || w != 12 {
		t.Fatalf("width=%v err=%v", w, err)
	}
	if _, err := getMetrics(nil, &recordedShaper{}); err != nil {
		t.Fatalf("EOF on empty run: %v", err)
	}
}

func TestClusterMetricErrors(t *testing.T) {
	shapeErr := errors.New("shaping failed")
	for _, records := range [][]metricRecord{
		{{-1, 1, nil}}, {{4, 1, nil}}, {{1, 1, nil}},
		{{3, 1, nil}, {2, 1, nil}}, {{0, 0, shapeErr}},
		{{0, math.MaxInt32, nil}, {0, 1, nil}},
	} {
		if _, err := getMetrics([]byte("é x"), &recordedShaper{records}); err == nil {
			t.Fatalf("accepted invalid metrics: %+v", records)
		}
	}
	m, err := getMetrics([]byte("ab"), &recordedShaper{[]metricRecord{{0, math.MaxInt32, nil}, {1, 1, nil}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.width(0, 2); err == nil {
		t.Fatal("accepted overflowing knot width")
	}
}

func TestNormalizeEncodingPenalties(t *testing.T) {
	for _, tc := range []struct {
		input int64
		want  Penalty
	}{
		{0, 0}, {550, 550}, {-240, -240},
		{9999, 9999}, {-9999, -9999},
		{10000, MaxPenalty}, {-10000, MaxMerit},
		{18000, MaxPenalty}, {-18000, MaxMerit},
		{math.MaxInt64, MaxPenalty}, {math.MinInt64, MaxMerit},
	} {
		if got := normPenalty(tc.input); got != tc.want {
			t.Errorf("%d: got %d want %d", tc.input, got, tc.want)
		}
	}
}
