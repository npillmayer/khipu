package khipu

import "testing"

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
