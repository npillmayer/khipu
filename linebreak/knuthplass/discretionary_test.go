package knuthplass

import (
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
)

type testDiscretionaryProvider struct {
	calls int
	want  []khipu.DiscretionaryCandidate
}

func (p *testDiscretionaryProvider) DiscretionaryCandidates(khp *khipu.Khipu, at int) ([]khipu.DiscretionaryCandidate, error) {
	p.calls++
	return p.want, nil
}

func TestDiscretionaryCandidatesFallbackToKhipuCache(t *testing.T) {
	khp := newKhipu([]node{
		{w: 20, minw: 20, maxw: 20, p: InfinityDemerits, kind: khipu.KTTextBox},
	})
	khp.AddDiscretionaryCandidate(0, khipu.DiscretionaryCandidate{
		Variant: 1,
		PreBreak: khipu.KnotCore{
			W: 8 * dimen.BP, MinW: 8 * dimen.BP, MaxW: 8 * dimen.BP, Kind: khipu.KTTextBox,
		},
		PostBreak: khipu.KnotCore{
			W: 12 * dimen.BP, MinW: 12 * dimen.BP, MaxW: 12 * dimen.BP, Kind: khipu.KTTextBox,
		},
	})
	kp := newLinebreaker(nil, NewKPDefaultParameters())
	got, err := kp.discretionaryCandidates(khp, 0)
	if err != nil {
		t.Fatalf("expected cached discretionary lookup to succeed: %v", err)
	}
	if len(got) != 1 || got[0].Variant != 1 {
		t.Fatalf("unexpected discretionary candidates: %+v", got)
	}
}

func TestDiscretionaryCandidatesDelegateToProvider(t *testing.T) {
	provider := &testDiscretionaryProvider{
		want: []khipu.DiscretionaryCandidate{{
			Variant: 4,
			PreBreak: khipu.KnotCore{
				W: 9 * dimen.BP, MinW: 9 * dimen.BP, MaxW: 9 * dimen.BP, Kind: khipu.KTTextBox,
			},
			PostBreak: khipu.KnotCore{
				W: 11 * dimen.BP, MinW: 11 * dimen.BP, MaxW: 11 * dimen.BP, Kind: khipu.KTTextBox,
			},
		}},
	}
	params := NewKPDefaultParameters()
	params.DiscretionaryProvider = provider
	khp := newKhipu([]node{
		{w: 20, minw: 20, maxw: 20, p: InfinityDemerits, kind: khipu.KTTextBox},
	})
	kp := newLinebreaker(nil, params)
	got, err := kp.discretionaryCandidates(khp, 0)
	if err != nil {
		t.Fatalf("expected provider lookup to succeed: %v", err)
	}
	if provider.calls != 1 {
		t.Fatalf("expected provider to be called once, got %d", provider.calls)
	}
	if len(got) != 1 || got[0].Variant != 4 {
		t.Fatalf("unexpected provider candidates: %+v", got)
	}
}
