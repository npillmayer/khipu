package khipu

import (
	"testing"

	"github.com/npillmayer/khipu/dimen"
)

func TestNewKhipukamayuqDefaultShaper(t *testing.T) {
	first, second := NewKhipukamayuq(Config{}), NewKhipukamayuq(Config{})
	for _, owner := range []*Khipukamayuq{first, second, first} {
		k, err := owner.EncodeParagraph(t.Context(), preparationParagraph(t, "x"))
		if err != nil {
			t.Fatal(err)
		}
		if k.W[0] != 5*dimen.PT || len(k.shaping.glyphs) != 1 {
			t.Fatal("default stream must be fresh with retained glyphs", k)
		}
	}
}
