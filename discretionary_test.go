package khipu

import (
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/uax/bidi"
)

type testhyphenator struct {
	calls int
	want  []HyphenationFragment
}

func (h *testhyphenator) Discretionaries(word string, params *Params) ([]HyphenationFragment, error) {
	h.calls++
	return h.want, nil
}

func TestKhipuStoresKnotFlags(t *testing.T) {
	k := &Khipu{
		W:       make([]dimen.DU, 0, 4),
		MinW:    make([]dimen.DU, 0, 4),
		MaxW:    make([]dimen.DU, 0, 4),
		Penalty: make([]Penalty, 0, 4),
		Pos:     make([]uint64, 0, 4),
		Len:     make([]uint16, 0, 4),
		Kind:    make([]KnotType, 0, 4),
		Flags:   make([]KnotFlags, 0, 4),
	}
	k.appendKnot(KnotCore{
		W: 10 * dimen.BP, MinW: 10 * dimen.BP, MaxW: 10 * dimen.BP,
		Kind: KTGlue, Flags: KFDiscardable,
	})
	k.appendKnot(KnotCore{
		W: 20 * dimen.BP, MinW: 20 * dimen.BP, MaxW: 20 * dimen.BP,
		Kind: KTTextBox,
	})
	if !k.KnotByIndex(0).IsDiscardable() {
		t.Fatalf("expected first knot to be discardable")
	}
	if k.KnotByIndex(1).IsDiscardable() {
		t.Fatalf("expected second knot not to be discardable")
	}
	if idx, ok := k.Discardable(0); !ok || idx != 0 {
		t.Fatalf("expected discardable scan to report first knot as discardable, got idx=%d ok=%v", idx, ok)
	}
}

func TestKhipuStoresDiscretionaries(t *testing.T) {
	k := &Khipu{
		W:       make([]dimen.DU, 0, 2),
		MinW:    make([]dimen.DU, 0, 2),
		MaxW:    make([]dimen.DU, 0, 2),
		Penalty: make([]Penalty, 0, 2),
		Pos:     make([]uint64, 0, 2),
		Len:     make([]uint16, 0, 2),
		Kind:    make([]KnotType, 0, 2),
		Flags:   make([]KnotFlags, 0, 2),
	}
	k.appendKnot(KnotCore{
		W: 30 * dimen.BP, MinW: 30 * dimen.BP, MaxW: 30 * dimen.BP,
		Kind: KTTextBox,
	})
	cand := DiscretionaryCandidate{
		Variant: 1,
		PreBreak: KnotCore{
			W: 15 * dimen.BP, MinW: 15 * dimen.BP, MaxW: 15 * dimen.BP, Kind: KTTextBox,
		},
		PostBreak: KnotCore{
			W: 15 * dimen.BP, MinW: 15 * dimen.BP, MaxW: 15 * dimen.BP, Kind: KTTextBox,
		},
	}
	if !k.AddDiscretionaryCandidate(0, cand) {
		t.Fatalf("expected discretionary candidate to be added")
	}
	ds := k.DiscretionariesAt(0)
	if len(ds) != 1 || ds[0].Variant != 1 {
		t.Fatalf("unexpected discretionary candidates: %+v", ds)
	}
	if !k.SelectDiscretionary(0, DiscretionarySelection{Source: 0, Variant: 1}) {
		t.Fatalf("expected discretionary selection to be stored")
	}
	choice, ok := k.SelectedDiscretionaryAt(0)
	if !ok || choice.Source != 0 || choice.Variant != 1 {
		t.Fatalf("unexpected discretionary selection %+v ok=%v", choice, ok)
	}
}

func TestKhipukamayuqProvidesCachedDiscretionaries(t *testing.T) {
	kq := newTestKq()
	k := kq.allocKhipu(1)
	k.appendKnot(KnotCore{
		W: 30 * dimen.BP, MinW: 30 * dimen.BP, MaxW: 30 * dimen.BP,
		Kind: KTTextBox,
	})
	cand := DiscretionaryCandidate{
		Variant: 2,
		PreBreak: KnotCore{
			W: 12 * dimen.BP, MinW: 12 * dimen.BP, MaxW: 12 * dimen.BP, Kind: KTTextBox,
		},
		PostBreak: KnotCore{
			W: 18 * dimen.BP, MinW: 18 * dimen.BP, MaxW: 18 * dimen.BP, Kind: KTTextBox,
		},
	}
	if !k.AddDiscretionaryCandidate(0, cand) {
		t.Fatalf("expected discretionary candidate to be added")
	}
	got, err := kq.DiscretionaryCandidates(k, 0)
	if err != nil {
		t.Fatalf("expected cached discretionary lookup to succeed: %v", err)
	}
	if len(got) != 1 || got[0].Variant != 2 {
		t.Fatalf("unexpected cached discretionary candidates: %+v", got)
	}
}

func TestKhipukamayuqShapesAndCachesHyphenatorDiscretionaries(t *testing.T) {
	env := typEnv{
		shaper: &testshaper{},
		params: newParameters(),
		hyphenator: &testhyphenator{
			want: []HyphenationFragment{{
				Variant:   1,
				PreBreak:  "break",
				PostBreak: "ing",
			}},
		},
	}
	kq := NewKhipukamayuq(env)
	text := styled.TextFromString("breaking")
	para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), bidi.LeftToRight, nil)
	if err != nil {
		t.Fatalf("ParagraphFromText failed: %v", err)
	}
	k := kq.allocKhipu(1)
	kq.acquire(k)
	k.paragraph = para
	k.appendKnot(KnotCore{
		Pos:  para.Offset,
		Len:  uint16(len("breaking")),
		Kind: KTTextBox,
	})
	got, err := kq.DiscretionaryCandidates(k, 0)
	if err != nil {
		t.Fatalf("expected discretionary lookup to succeed: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 discretionary candidate, got %+v", got)
	}
	if got[0].PreBreak.W != 25*dimen.PT || got[0].PostBreak.W != 15*dimen.PT {
		t.Fatalf("unexpected shaped discretionary widths: %+v", got[0])
	}
	if got[0].PreBreak.Penalty != Penalty(env.params.Hyphenpenalty) {
		t.Fatalf("expected discretionary penalty %d, got %d", env.params.Hyphenpenalty, got[0].PreBreak.Penalty)
	}
	if len(k.DiscretionariesAt(0)) != 1 {
		t.Fatalf("expected discretionary candidate to be cached on khipu")
	}
	if env.hyphenator.(*testhyphenator).calls != 1 {
		t.Fatalf("expected hyphenator to be called once, got %d", env.hyphenator.(*testhyphenator).calls)
	}
	got, err = kq.DiscretionaryCandidates(k, 0)
	if err != nil {
		t.Fatalf("expected cached discretionary lookup to succeed: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected cached discretionary candidate, got %+v", got)
	}
	if env.hyphenator.(*testhyphenator).calls != 1 {
		t.Fatalf("expected cached lookup not to call hyphenator again, got %d", env.hyphenator.(*testhyphenator).calls)
	}
}
