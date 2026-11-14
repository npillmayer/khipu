package khipu_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak/firstfit"
	"github.com/npillmayer/khipu/linebreak/knuthplass"
	"github.com/npillmayer/uax/bidi"
)

type atomicShape dimen.DU

func (s atomicShape) LineLength(int32) dimen.DU { return dimen.DU(s) }

func TestAtomicBothBreakersThroughFreeze(t *testing.T) {
	for _, engine := range []string{"firstfit", "knuthplass"} {
		for _, tc := range []struct {
			name, text string
			dir        bidi.Direction
			advances   []dimen.DU
			width      dimen.DU
			wantOrder  []contracts.AtomicID
			spaces     int
			overfull   bool
		}{
			{"object only", "\uFFFC", bidi.LeftToRight, []dimen.DU{30}, 50, []contracts.AtomicID{1}, 0, false},
			{"zero", "\uFFFC\uFFFC", bidi.LeftToRight, []dimen.DU{0, 0}, 0, []contracts.AtomicID{1, 2}, 0, false},
			{"overfull", "\uFFFC", bidi.LeftToRight, []dimen.DU{70}, 20, []contracts.AtomicID{1}, 0, true},
			{"zero overfull target", "\uFFFC", bidi.LeftToRight, []dimen.DU{70}, 0, []contracts.AtomicID{1}, 0, true},
			{"spaces", " \uFFFC a \uFFFC ", bidi.LeftToRight, []dimen.DU{0, 0}, 100 * dimen.PT, []contracts.AtomicID{1, 2}, 2, false},
			{"mixed bidi", "a אב\uFFFC\uFFFCג z", bidi.LeftToRight, []dimen.DU{0, 20}, 100 * dimen.PT, []contracts.AtomicID{2, 1}, 2, false},
			{"rtl atoms", "\uFFFC\uFFFC", bidi.RightToLeft, []dimen.DU{10, 20}, 40, []contracts.AtomicID{2, 1}, 0, false},
			{"signed", "\uFFFC\uFFFC", bidi.LeftToRight, []dimen.DU{-10, 30}, 20, []contracts.AtomicID{1, 2}, 0, false},
			{"wide prefix", "\uFFFC\uFFFC\uFFFC", bidi.LeftToRight, []dimen.DU{1500000000, 1500000000, -1500000000}, 1500000000, []contracts.AtomicID{1, 2, 3}, 0, false},
		} {
			t.Run(engine+"/"+tc.name, func(t *testing.T) {
				text := styled.TextFromString(tc.text)
				para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), tc.dir, nil)
				if err != nil {
					t.Fatal(err)
				}
				var anchors []contracts.AtomicAnchor
				var advances []khipu.AtomicAdvance
				var overrides []khipu.AtomicBoundary
				for pos, r := range tc.text {
					if r != '\uFFFC' {
						continue
					}
					id := contracts.AtomicID(len(anchors) + 1)
					anchors = append(anchors, contracts.AtomicAnchor{ID: id, ByteOffset: uint64(pos)})
					advances = append(advances, khipu.AtomicAdvance{ID: id, Advance: tc.advances[id-1]})
					// Keep adjacent objects together to test complete signed sums.
					if len(anchors) > 1 && anchors[len(anchors)-2].ByteOffset+3 == uint64(pos) {
						overrides = append(overrides, khipu.AtomicBoundary{ByteOffset: uint64(pos), Action: khipu.ProhibitBoundary})
					}
				}
				owner := khipu.NewKhipukamayuq(khipu.Config{})
				prepared, err := owner.PrepareParagraph(t.Context(), khipu.ParagraphInput{Paragraph: para, Atomics: anchors}, khipu.PrepareOptions{AtomicBoundaries: overrides})
				if err != nil {
					t.Fatal(err)
				}
				slices.Reverse(advances) // Dimension order is independent of source order.
				k, err := owner.BindParagraph(t.Context(), prepared, advances)
				if err != nil {
					t.Fatal(err)
				}
				layout := khipu.Params{LineShape: atomicShape(tc.width)}
				var breaks []int
				if engine == "firstfit" {
					breaks, err = firstfit.BreakParagraph(k, layout, nil)
				} else {
					breaks, err = knuthplass.BreakParagraph(k, layout, nil)
				}
				if err != nil {
					t.Fatal(err)
				}
				f, err := owner.Freeze(t.Context(), k, breaks, layout)
				if err != nil {
					t.Fatal(err)
				}
				if f.LineCount() != 1 {
					t.Fatalf("want one line, got %d: %v", f.LineCount(), breaks)
				}
				var order []contracts.AtomicID
				spaces := 0
				for i := 0; i < f.KnotCount(); i++ {
					knot, _ := f.Knot(i)
					if knot.Kind == khipu.KTGlue && knot.Material != khipu.ParagraphFillMaterial {
						spaces++
					}
					id, ok := f.Atomic(i)
					if !ok {
						if knot.Kind == khipu.KTAtomic {
							t.Fatal("lost atomic ID")
						}
						continue
					}
					order = append(order, id)
					if knot.W != tc.advances[id-1] || knot.Source.Bytes.Start != anchors[id-1].ByteOffset || knot.Source.Bytes.End != anchors[id-1].ByteOffset+3 || knot.Source.ScalarEnd-knot.Source.ScalarStart != 1 {
						t.Fatal("lost atomic geometry/provenance", knot)
					}
					if _, ok := f.CopyText(i); ok {
						t.Fatal("atomic has shaped text")
					}
				}
				if !slices.Equal(order, tc.wantOrder) || spaces != tc.spaces {
					t.Fatalf("order=%v spaces=%d", order, spaces)
				}
				line, _ := f.Line(0)
				if (line.Fit == khipu.FitOverfull) != tc.overfull {
					t.Fatalf("fit=%v", line)
				}
				if _, ok := f.Atomic(-1); ok {
					t.Fatal("invalid index")
				}
				if _, ok := f.Atomic(f.KnotCount()); ok {
					t.Fatal("invalid index")
				}
			})
		}
	}
}

func TestAtomicLineBoundariesAndSignedCheckpoint(t *testing.T) {
	for _, engine := range []string{"firstfit", "knuthplass"} {
		for _, tc := range []struct {
			name, text string
			width      dimen.DU
			advances   []dimen.DU
			bounds     []khipu.AtomicBoundary
			lines      int
		}{
			{"unicode", "\uFFFC\uFFFC\uFFFC", 20, []dimen.DU{20, 20, 20}, nil, 3},
			{"explicit", "\uFFFC\uFFFC\uFFFC", 20, []dimen.DU{20, 20, 20}, []khipu.AtomicBoundary{{ByteOffset: 3, Action: khipu.AllowBoundary}, {ByteOffset: 6, Action: khipu.AllowBoundary}}, 3},
			{"mandatory", "\uFFFC\n\uFFFC", 50, []dimen.DU{20, 20}, nil, 2},
			{"signed checkpoint", "\uFFFC\uFFFC\uFFFC\uFFFC", 1500000000, []dimen.DU{1500000000, 1500000000, -1500000000, 1500000000}, []khipu.AtomicBoundary{{ByteOffset: 3, Action: khipu.ProhibitBoundary}, {ByteOffset: 6, Action: khipu.ProhibitBoundary}, {ByteOffset: 9, Action: khipu.AllowBoundary}}, 2},
		} {
			t.Run(engine+"/"+tc.name, func(t *testing.T) {
				text := styled.TextFromString(tc.text)
				para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), bidi.LeftToRight, nil)
				if err != nil {
					t.Fatal(err)
				}
				var anchors []contracts.AtomicAnchor
				var advances []khipu.AtomicAdvance
				for pos, r := range tc.text {
					if r == '\uFFFC' {
						id := contracts.AtomicID(len(anchors) + 1)
						anchors = append(anchors, contracts.AtomicAnchor{ID: id, ByteOffset: uint64(pos)})
						advances = append(advances, khipu.AtomicAdvance{ID: id, Advance: tc.advances[id-1]})
					}
				}
				owner := khipu.NewKhipukamayuq(khipu.Config{})
				p, err := owner.PrepareParagraph(t.Context(), khipu.ParagraphInput{Paragraph: para, Atomics: anchors}, khipu.PrepareOptions{AtomicBoundaries: tc.bounds})
				if err != nil {
					t.Fatal(err)
				}
				k, err := owner.BindParagraph(t.Context(), p, advances)
				if err != nil {
					t.Fatal(err)
				}
				layout := khipu.Params{LineShape: atomicShape(tc.width)}
				var breaks []int
				if engine == "firstfit" {
					breaks, err = firstfit.BreakParagraph(k, layout, nil)
				} else {
					breaks, err = knuthplass.BreakParagraph(k, layout, nil)
				}
				if err != nil {
					t.Fatal(err)
				}
				f, err := owner.Freeze(t.Context(), k, breaks, layout)
				if err != nil {
					t.Fatal(err)
				}
				if f.LineCount() != tc.lines {
					t.Fatalf("breaks %v, want %d lines", breaks, tc.lines)
				}
				var seen []contracts.AtomicID
				for i := 0; i < f.KnotCount(); i++ {
					if id, ok := f.Atomic(i); ok {
						seen = append(seen, id)
					}
				}
				for i, id := range seen {
					if id != contracts.AtomicID(i+1) {
						t.Fatal("lost source order", seen)
					}
				}
				if len(seen) != len(anchors) {
					t.Fatal("lost or duplicated atomic", seen)
				}
				// A frozen result owns its identity/widths after a working mutation
				// and a later bind. Source remains borrowed under the contract.
				for i := range k.W {
					k.W[i] = 0
					k.Kind[i] = khipu.KTGlue
				}
				if _, err := owner.BindParagraph(t.Context(), p, advances); err != nil {
					t.Fatal(err)
				}
				for i := 0; i < f.KnotCount(); i++ {
					if id, ok := f.Atomic(i); ok {
						knot, _ := f.Knot(i)
						if knot.W != tc.advances[id-1] {
							t.Fatal("frozen snapshot mutated")
						}
					}
				}
			})
		}
	}
}

func TestAtomicNegativeCompletedLine(t *testing.T) {
	text := styled.TextFromString("\uFFFC")
	para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), bidi.LeftToRight, nil)
	if err != nil {
		t.Fatal(err)
	}
	owner := khipu.NewKhipukamayuq(khipu.Config{})
	p, err := owner.PrepareParagraph(t.Context(), khipu.ParagraphInput{Paragraph: para, Atomics: []contracts.AtomicAnchor{{ID: 1}}}, khipu.PrepareOptions{})
	if err != nil {
		t.Fatal(err)
	}
	k, err := owner.BindParagraph(t.Context(), p, []khipu.AtomicAdvance{{ID: 1, Advance: -10}})
	if err != nil {
		t.Fatal(err)
	}
	for _, engine := range []string{"firstfit", "knuthplass"} {
		layout := khipu.Params{LineShape: atomicShape(0)}
		var breaks []int
		if engine == "firstfit" {
			breaks, err = firstfit.BreakParagraph(k, layout, nil)
		} else {
			breaks, err = knuthplass.BreakParagraph(k, layout, nil)
		}
		if err != nil {
			t.Fatal(err)
		}
		if f, err := owner.Freeze(t.Context(), k, breaks, layout); f != nil || !errors.Is(err, khipu.ErrUnsupportedPolicy) {
			t.Fatal(f, err)
		}
	}
}
