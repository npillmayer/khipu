package khipu_test

import (
	"context"
	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
	"github.com/npillmayer/uax/bidi"
	"testing"
)

type retentionFace struct {
	helpers.SyntheticFont
	calls int
}

func (f *retentionFace) Scale(size dimen.DU) (contracts.FontScale, error) {
	f.calls++
	return f.SyntheticFont.Scale(size)
}

type retentionStream struct{ contracts.StreamingShaper }

func (s retentionStream) CheckFace(f contracts.FontFace) error { return nil }
func (s retentionStream) Shape(ctx context.Context, p contracts.ShapeParams, r contracts.RuneReader, o contracts.ShapeBufferOptions) error {
	// The synthetic backend consumes its own representation; the owner retains
	// the original counting face. No calls reach that face through this bridge.
	i, err := contracts.NewFontInstance(helpers.NewSyntheticFont(), p.Font.EmSize())
	if err != nil {
		return err
	}
	p.Font = i
	return s.StreamingShaper.Shape(ctx, p, r, o)
}
func TestInstanceRetainedThroughMeasureBindFreeze(t *testing.T) {
	f := new(retentionFace)
	owner := khipu.NewKhipukamayuq(khipu.Config{Streaming: &khipu.StreamingConfig{
		Defaults: contracts.ResolvedRunStyle{Font: f, EmSize: 12 * dimen.BP, Spacing: contracts.SpacingElastic},
		Resolver: helpers.DefaultStyleResolver{},
		NewShaper: func() (contracts.StreamingShaper, error) {
			s, e := helpers.NewSyntheticStreamingShaper(8)
			return retentionStream{s}, e
		},
	}})
	text := styled.TextFromString("ab cd ef")
	para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), bidi.LeftToRight, nil)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := owner.PrepareParagraphFixture(khipu.ParagraphInput{Paragraph: para}, khipu.PrepareOptions{})
	if err != nil {
		t.Fatal(err)
	}
	calls := f.calls
	if calls != 1 {
		t.Fatalf("uniform run bound %d times", calls)
	}
	for _, kp := range []bool{false, true} {
		if _, err := owner.MeasureIntrinsicFixture(prepared, khipu.MeasureRequest{}); err != nil {
			t.Fatal(err)
		}
		elastic, err := owner.BindParagraphFixture(prepared, nil)
		if err != nil {
			t.Fatal(err)
		}
		params := khipu.Params{LineShape: atomicShape(30 * dimen.BP)}
		var breaks []int
		if kp {
			breaks, err = knuthplassFixture(elastic, params, nil)
		} else {
			breaks, err = firstfitFixture(elastic, params, nil)
		}
		if err != nil {
			t.Fatal(err)
		}
		frozen, err := owner.FreezeFixture(elastic, breaks, params)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for n := 0; n < frozen.KnotCount(); n++ {
			a, ok := frozen.CopyText(n)
			if !ok {
				continue
			}
			found = true
			inst := a.Context.Params.Font
			if !inst.Valid() || inst.Face() != f || inst.EmSize() != 12*dimen.BP || inst.Scale().ToDU(500) != float64(6*dimen.BP) {
				t.Fatal("lost instance")
			}
			a.Context.Params.Font = contracts.FontInstance{}
			again, _ := frozen.CopyText(n)
			if !again.Context.Params.Font.Valid() {
				t.Fatal("copy changed frozen binding")
			}
		}
		if !found || f.calls != calls {
			t.Fatal("bind/freeze/copy rebuilt instance", f.calls)
		}
	}
}
