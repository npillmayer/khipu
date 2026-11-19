package khipu_test

import (
	"errors"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/khipu/linebreak/firstfit"
	"github.com/npillmayer/uax/bidi"
)

func TestPublicFreezeValidationAndRetry(t *testing.T) {
	for _, engine := range []string{"firstfit", "knuthplass"} {
		for _, dir := range []bidi.Direction{bidi.LeftToRight, bidi.RightToLeft} {
			t.Run(engine+dir.String(), func(t *testing.T) {
				text := styled.TextFromString("aa\nbb")
				para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), dir, nil)
				if err != nil {
					t.Fatal(err)
				}
				owner := khipu.NewKhipukamayuq(khipu.Config{})
				k, err := owner.EncodeParagraphFixture(para)
				if err != nil {
					t.Fatal(err)
				}
				layout := firstfit.DefaultLayout(linebreak.RectangularParShape(30 * dimen.PT))
				var breaks []int
				if engine == "firstfit" {
					breaks, err = firstfitFixture(k, layout, nil)
				} else {
					breaks, err = knuthplassFixture(k, layout, nil)
				}
				if err != nil {
					t.Fatal(err)
				}
				layout.LineShape = linebreak.RectangularParShape(30 * dimen.PT)
				f, err := owner.FreezeFixture(k, breaks, layout)
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
				if f, err := (*khipu.Khipukamayuq)(nil).FreezeFixture(k, breaks, layout); f != nil || !errors.Is(err, khipu.ErrVoidKhipukamayuq) {
					t.Fatal(f, err)
				}
				foreign := khipu.NewKhipukamayuq(khipu.Config{})
				if f, err := foreign.FreezeFixture(k, breaks, layout); f != nil || !errors.Is(err, khipu.ErrIllegalArguments) {
					t.Fatal(f, err)
				}
				if f, err := owner.FreezeFixture(k, nil, layout); f != nil || !errors.Is(err, khipu.ErrIllegalArguments) {
					t.Fatal(f, err)
				}
				// A valid but unsupported input is still an atomic failure, unlike
				// an overfull line, which is a successful physical outcome.
				k.MaxW[0]++
				if f, err := owner.FreezeFixture(k, breaks, layout); f != nil || !errors.Is(err, khipu.ErrUnsupportedAdjustment) {
					t.Fatal(f, err)
				}
				k.MaxW[0]--
				if _, err := owner.FreezeFixture(k, breaks, layout); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
