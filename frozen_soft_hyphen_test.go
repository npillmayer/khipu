package khipu_test

import (
	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/khipu/linebreak/firstfit"
	"github.com/npillmayer/khipu/linebreak/knuthplass"
	"github.com/npillmayer/uax/bidi"
	"io"
	"reflect"
	"testing"
)

func TestFreezeUnbrokenSoftHyphenRTL(t *testing.T) {
	for _, engine := range []string{"firstfit", "knuthplass"} {
		t.Run(engine, func(t *testing.T) {
			source := "a\u00adb"
			text := styled.TextFromString(source)
			p, err := styled.ParagraphFromText(&text, 0, uint64(len(source)), bidi.RightToLeft, nil)
			if err != nil {
				t.Fatal(err)
			}
			owner := khipu.NewKhipukamayuq(khipu.Config{})
			k, err := owner.EncodeParagraph(t.Context(), p)
			if err != nil {
				t.Fatal(err)
			}
			shape := linebreak.RectangularParShape(200 * dimen.BP)
			layout := firstfit.DefaultLayout(shape)
			var breaks []int
			if engine == "firstfit" {
				breaks, err = firstfit.BreakParagraph(k, layout, nil)
			} else {
				layout = knuthplass.DefaultLayout(shape)
				breaks, err = knuthplass.BreakParagraph(k, layout, nil)
			}
			if err != nil {
				t.Fatal(err)
			}
			f, err := owner.Freeze(t.Context(), k, breaks, layout)
			if err != nil {
				t.Fatal(err)
			}
			if f.LineCount() != 1 || f.SourceLen() != uint64(len(source)) {
				t.Fatal("source/line coverage changed")
			}
			var glyphs, clusters []uint32
			for i := 0; i < f.KnotCount(); i++ {
				if text, ok := f.CopyText(i); ok {
					for _, g := range text.Glyphs {
						glyphs = append(glyphs, g.GlyphID)
						clusters = append(clusters, g.Cluster)
					}
				}
			}
			if !reflect.DeepEqual(glyphs, []uint32{'a' + 1, 'b' + 1}) || !reflect.DeepEqual(clusters, []uint32{0, 2}) {
				t.Fatalf("physical glyphs %v clusters %v", glyphs, clusters)
			}
			r, err := f.SourceReader(khipu.ByteRange{Start: 0, End: uint64(len(source))})
			if err != nil {
				t.Fatal(err)
			}
			data, err := io.ReadAll(r)
			if err != nil || string(data) != source {
				t.Fatal("original source lost")
			}
		})
	}
}
