package khipu_test

import (
	"context"
	"errors"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/khipu/linebreak/firstfit"
	"github.com/npillmayer/khipu/linebreak/knuthplass"
	"github.com/npillmayer/uax/bidi"
)

type cancelFreezeShape struct {
	cancel context.CancelFunc
	calls  int
}

func (s *cancelFreezeShape) LineLength(int32) dimen.DU {
	s.calls++
	if s.calls == 2 {
		s.cancel()
	}
	return 30 * dimen.PT
}

func TestPublicFreezeCancellationAndRetry(t *testing.T) {
	for _, engine := range []string{"firstfit", "knuthplass"} {
		for _, dir := range []bidi.Direction{bidi.LeftToRight, bidi.RightToLeft} {
			t.Run(engine+dir.String(), func(t *testing.T) {
				text := styled.TextFromString("aa\nbb")
				para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), dir, nil)
				if err != nil {
					t.Fatal(err)
				}
				owner := khipu.NewKhipukamayuq(khipu.Config{})
				k, err := owner.EncodeParagraph(t.Context(), para)
				if err != nil {
					t.Fatal(err)
				}
				layout := firstfit.DefaultLayout(linebreak.RectangularParShape(30 * dimen.PT))
				var breaks []int
				if engine == "firstfit" {
					breaks, err = firstfit.BreakParagraph(k, layout, nil)
				} else {
					breaks, err = knuthplass.BreakParagraph(k, layout, nil)
				}
				if err != nil {
					t.Fatal(err)
				}
				w, min, max := slices.Clone(k.W), slices.Clone(k.MinW), slices.Clone(k.MaxW)
				selected := maps.Clone(k.SelectedDiscretionaries)
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				shape := &cancelFreezeShape{cancel: cancel}
				layout.LineShape = shape
				f, err := owner.Freeze(ctx, k, breaks, layout)
				if f != nil || !errors.Is(err, context.Canceled) || shape.calls != 2 {
					t.Fatal(f, err, shape.calls)
				}
				if !slices.Equal(w, k.W) || !slices.Equal(min, k.MinW) || !slices.Equal(max, k.MaxW) || !maps.Equal(selected, k.SelectedDiscretionaries) {
					t.Fatal("cancel mutated input")
				}
				layout.LineShape = linebreak.RectangularParShape(30 * dimen.PT)
				f, err = owner.Freeze(t.Context(), k, breaks, layout)
				if err != nil || f.LineCount() != 2 {
					t.Fatal(f, err)
				}
				for i := 0; i < f.LineCount(); i++ {
					line, _ := f.Line(i)
					sum := line.LeftSkip + line.RightSkip
					for j := 0; j < line.KnotCount; j++ {
						knot, _ := f.Knot(line.FirstKnot + j)
						sum += knot.W
					}
					if sum != line.Width || line.Width != line.TargetWidth {
						t.Fatal(line)
					}
				}
				for _, expired := range []context.Context{ctx, expiredFreezeContext(t)} {
					if f, err := owner.Freeze(expired, k, breaks, layout); f != nil || !errors.Is(err, expired.Err()) {
						t.Fatal(f, err)
					}
				}
				if f, err := owner.Freeze(nil, k, breaks, layout); f != nil || !errors.Is(err, khipu.ErrIllegalArguments) {
					t.Fatal(f, err)
				}
				if f, err := (*khipu.Khipukamayuq)(nil).Freeze(t.Context(), k, breaks, layout); f != nil || !errors.Is(err, khipu.ErrVoidKhipukamayuq) {
					t.Fatal(f, err)
				}
				foreign := khipu.NewKhipukamayuq(khipu.Config{})
				if f, err := foreign.Freeze(t.Context(), k, breaks, layout); f != nil || !errors.Is(err, khipu.ErrIllegalArguments) {
					t.Fatal(f, err)
				}
				if f, err := owner.Freeze(t.Context(), k, nil, layout); f != nil || !errors.Is(err, khipu.ErrIllegalArguments) {
					t.Fatal(f, err)
				}
				// A valid but unsupported input is still an atomic failure, unlike
				// an overfull line, which is a successful physical outcome.
				k.MaxW[0]++
				if f, err := owner.Freeze(t.Context(), k, breaks, layout); f != nil || !errors.Is(err, khipu.ErrUnsupportedAdjustment) {
					t.Fatal(f, err)
				}
				k.MaxW[0]--
				if _, err := owner.Freeze(t.Context(), k, breaks, layout); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func expiredFreezeContext(t *testing.T) context.Context {
	ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	t.Cleanup(cancel)
	return ctx
}
