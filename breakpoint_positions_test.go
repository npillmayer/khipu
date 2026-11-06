package khipu

import (
	"errors"
	"reflect"
	"testing"

	"github.com/npillmayer/khipu/contracts"
)

func TestBreakpointPositionsOrdinary(t *testing.T) {
	for _, text := range []string{"", "é beta", "a  b", "אב cd"} {
		p := preparationParagraph(t, text)
		p.Offset = 100 // Pos is absolute; the result must not be.
		owner := automaticTestOwner(nil, nil)
		k, err := owner.EncodeParagraph(t.Context(), p)
		if err != nil {
			t.Fatal(err)
		}
		breaks := []int{}
		want := []uint64{}
		for i, penalty := range k.Penalty {
			if penalty < MaxPenalty {
				breaks = append(breaks, i)
				want = append(want, k.Pos[i]-p.Offset+uint64(k.Len[i]))
			}
		}
		got, err := k.BreakpointPositions(breaks)
		if err != nil || !reflect.DeepEqual(got, want) || got[len(got)-1] != uint64(len(text)) {
			t.Fatalf("text=%q: got %v, want %v: %v", text, got, want, err)
		}
		if len(text) > 0 && len(got) > 1 && got[0] >= uint64(len(text)) {
			t.Fatal("ordinary interior break lost")
		}
		if got, err := k.BreakpointPositions(nil); err != nil || len(got) != 0 {
			t.Fatal(got, err)
		}
		if got, err := k.BreakpointPositions(breaks[:1]); err != nil || !reflect.DeepEqual(got, want[:1]) {
			t.Fatal("partial list", got, err)
		}
	}
}

func TestBreakpointPositionsGlyphlessGap(t *testing.T) {
	a, g := candidateFixture(t, "a\u2067אב\u2069z")
	// As in the candidate-stage test, force a boundary inside a silent run.
	// The first textbox ends at byte 1, but its break ends after the Hebrew text.
	a.boundaries[3].penalty = 0
	knots := accumulateTestGlyphs(t, a, g, []contracts.ShapedGlyph{{Cluster: 0, XAdvance: 500}, {Cluster: 5, XAdvance: 500}})
	k, err := NewKhipukamayuq(Config{}).publishCandidateParagraph(&candidateParagraph{a.para, a.runs, a.boundaries, knots})
	if err != nil {
		t.Fatal(err)
	}
	got, err := k.BreakpointPositions([]int{0, len(k.Kind) - 1})
	if err != nil || !reflect.DeepEqual(got, []uint64{8, 12}) || k.Len[0] != 1 {
		t.Fatal(got, err)
	}
}

func TestBreakpointPositionsBeforeSuppressedMarker(t *testing.T) {
	k, err := automaticTestOwner(nil, nil).EncodeParagraph(t.Context(), preparationParagraph(t, "a \u00adb"))
	if err != nil {
		t.Fatal(err)
	}
	for at, source := range k.shaping.sources {
		if source.breakAfter.byteEnd == 2 && k.Penalty[at] < MaxPenalty {
			got, err := k.BreakpointPositions([]int{at})
			if err != nil || !reflect.DeepEqual(got, []uint64{2}) {
				t.Fatal(got, err)
			}
			return
		}
	}
	t.Fatal("missing pre-marker space break")
}

func TestBreakpointPositionsSelectedProvenance(t *testing.T) {
	owner := automaticTestOwner(&testhyphenator{want: []HyphenationFragment{
		{Variant: 9, PreBreak: "LONG-", PostBreak: "lair", SourceSplit: SourceSplit{3, true}},
		{Variant: 4, PreBreak: "E-", PostBreak: "clair", SourceSplit: SourceSplit{2, true}},
	}}, nil)
	p := preparationParagraph(t, "xx éclair")
	p.Offset = 100
	k, err := owner.EncodeParagraph(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	candidates, err := owner.DiscretionaryCandidates(k, 2)
	if err != nil || len(candidates) != 2 {
		t.Fatal(candidates, err)
	}
	for _, tc := range []struct {
		variant uint16
		end     uint64
	}{{4, 5}, {9, 6}} {
		k.SelectDiscretionary(2, DiscretionarySelection{Source: 2, Variant: tc.variant})
		before := k.shaping
		got, err := k.BreakpointPositions([]int{2, len(k.Kind) - 1})
		if err != nil || !reflect.DeepEqual(got, []uint64{tc.end, 10}) {
			t.Fatal(got, err)
		}
		if !reflect.DeepEqual(before, k.shaping) || !reflect.DeepEqual(candidates, k.DiscretionariesAt(2)) || k.SelectedDiscretionaries[2].Variant != tc.variant {
			t.Fatal("lookup mutated source or decisions")
		}
	}
}

func TestBreakpointPositionsRejectInvalidInput(t *testing.T) {
	for _, mode := range []string{"negative", "outside", "duplicate index", "reversed", "missing source", "missing variant", "duplicate variant", "zero variant", "unknown split", "inside rune", "split at start", "split at end", "invalid source", "missing provenance", "bad boundary", "decreasing boundary"} {
		t.Run(mode, func(t *testing.T) {
			k := eligibilityParagraph(t, preparationParagraph(t, "a éclair"))
			k.AddDiscretionaryCandidate(2, DiscretionaryCandidate{Variant: 1, SourceSplit: SourceSplit{4, true}})
			k.SelectDiscretionary(2, DiscretionarySelection{Source: 2, Variant: 1})
			breaks := []int{1, 2, len(k.Kind) - 1}
			switch mode {
			case "negative":
				breaks[0] = -1
			case "outside":
				breaks[2] = len(k.Kind)
			case "duplicate index":
				breaks[1] = 1
			case "reversed":
				breaks = []int{2, 1}
			case "missing source":
				k.SelectedDiscretionaries[2] = DiscretionarySelection{Source: 99, Variant: 1}
			case "missing variant":
				delete(k.DiscretionaryCandidates, 2)
			case "duplicate variant":
				k.AddDiscretionaryCandidate(2, k.DiscretionariesAt(2)[0])
			case "zero variant":
				k.SelectedDiscretionaries[2] = DiscretionarySelection{Source: 2}
			case "unknown split":
				k.DiscretionaryCandidates[2][0].SourceSplit.Valid = false
			case "inside rune":
				k.DiscretionaryCandidates[2][0].SourceSplit.ByteOffset = 3
			case "split at start":
				k.DiscretionaryCandidates[2][0].SourceSplit.ByteOffset = 2
			case "split at end":
				k.DiscretionaryCandidates[2][0].SourceSplit.ByteOffset = 9
			case "invalid source":
				k.shaping.sources[2].source.byteEnd = 99
			case "missing provenance":
				k.shaping.sources = k.shaping.sources[:1]
			case "bad boundary":
				k.shaping.sources[1].breakAfter.byteEnd = 3
			case "decreasing boundary":
				k.shaping.sources[1].breakAfter.byteEnd = 5
			}
			if got, err := k.BreakpointPositions(breaks); got != nil || !errors.Is(err, ErrIllegalArguments) {
				t.Fatal("expected atomic failure", got, err)
			}
		})
	}
	for _, k := range []*Khipu{nil, {}, {paragraph: preparationParagraph(t, "")}} {
		if got, err := k.BreakpointPositions(nil); got != nil || !errors.Is(err, ErrIllegalArguments) {
			t.Fatal(got, err)
		}
	}
}

func TestBreakpointPositionsRequireDiscretionaryProvenance(t *testing.T) {
	k, err := NewKhipukamayuq(Config{}).EncodeParagraph(t.Context(), preparationParagraph(t, "éclair"))
	if err != nil {
		t.Fatal(err)
	}
	k.AddDiscretionaryCandidate(0, DiscretionaryCandidate{Variant: 1})
	k.SelectDiscretionary(0, DiscretionarySelection{Source: 0, Variant: 1})
	if got, err := k.BreakpointPositions([]int{0}); got != nil || !errors.Is(err, ErrIllegalArguments) {
		t.Fatal("guessed replacement position without SourceSplit", got, err)
	}
	k.DiscretionaryCandidates[0][0].SourceSplit = SourceSplit{1, true}
	if _, err := k.BreakpointPositions([]int{0}); !errors.Is(err, ErrIllegalArguments) {
		t.Fatal("accepted continuation byte", err)
	}
	k.DiscretionaryCandidates[0][0].SourceSplit = SourceSplit{2, true}
	if got, err := k.BreakpointPositions([]int{0}); err != nil || !reflect.DeepEqual(got, []uint64{2}) {
		t.Fatal(got, err)
	}
}

func TestBreakpointPositionsRejectMissingProvenance(t *testing.T) {
	k, err := NewKhipukamayuq(Config{}).EncodeParagraph(t.Context(), preparationParagraph(t, "a b"))
	if err != nil {
		t.Fatal(err)
	}
	// Plausible Pos/Len planes are not a substitute for boundary provenance.
	k.shaping.sources = nil
	for _, breaks := range [][]int{nil, {1}, {len(k.Kind) - 1}} {
		if got, err := k.BreakpointPositions(breaks); got != nil || !errors.Is(err, ErrIllegalArguments) {
			t.Fatalf("mapped knots without provenance: %v, %v", got, err)
		}
	}
}
