package khipu

import (
	"testing"

	"github.com/npillmayer/khipu/dimen"
)

func TestConfigGeometrySnapshot(t *testing.T) {
	shape := &geometryIntegrationShapeForConfig{width: 20 * dimen.PT}
	p := DefaultParams()
	p.LineShape = shape
	p.RightSkip = KnotCore{MaxW: dimen.Fil}
	owner := NewKhipukamayuq(Config{Params: &p})
	p.RightSkip = KnotCore{}
	p.LineShape = nil
	if owner.env.params.RightSkip.MaxW != dimen.Fil || owner.env.params.LineShape != shape {
		t.Fatal("configuration did not snapshot geometry values")
	}
	// Interface services are deliberately not deep-copied. Callers must keep
	// their behavior stable between breaking and freezing.
	shape.width++
	if owner.env.params.LineShape.LineLength(1) != shape.width {
		t.Fatal("unexpected shape copy")
	}
}

type geometryIntegrationShapeForConfig struct{ width dimen.DU }

func (s *geometryIntegrationShapeForConfig) LineLength(int32) dimen.DU { return s.width }

func TestConfigParameterSnapshot(t *testing.T) {
	for _, tc := range []struct {
		name    string
		params  *Params
		penalty Penalty
	}{
		{name: "default parameters", penalty: 10},
		{name: "explicit parameters", params: &Params{Hyphenpenalty: 37}, penalty: 37},
		{name: "explicit zero parameters", params: &Params{}, penalty: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			kq := NewKhipukamayuq(Config{
				Params: tc.params,
			})
			if tc.params != nil {
				tc.params.Hyphenpenalty = 99
			}
			if got := kq.env.params.Hyphenpenalty; got != int(tc.penalty) {
				t.Fatalf("expected construction-time penalty %d, got %d", tc.penalty, got)
			}
		})
	}
}
