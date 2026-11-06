package khipu

import (
	"context"
	"testing"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
)

func TestDefaultStreamingOwnerIsolation(t *testing.T) {
	first, second := NewKhipukamayuq(Config{}), NewKhipukamayuq(Config{})
	if first.env.streaming == second.env.streaming {
		t.Fatal("shared mutable configuration")
	}
	first.env.streaming.Defaults.EmSize = 20 * dimen.PT
	for i, owner := range []*Khipukamayuq{first, second} {
		for range 2 {
			k, err := owner.EncodeParagraph(t.Context(), preparationParagraph(t, "a"))
			if err != nil {
				t.Fatal(err)
			}
			want := dimen.DU(10) * dimen.PT
			if i == 1 {
				want = 5 * dimen.PT
			}
			if k.W[0] != want || len(k.shaping.boxes) != 1 || len(k.shaping.glyphs) != 1 {
				t.Fatal("lost default configuration or shaping output", k)
			}
		}
	}
	if k, err := second.EncodeParagraph(t.Context(), preparationParagraph(t, "")); err != nil || len(k.Kind) != 1 || k.Flags[0] != 0 || k.Penalty[0] != MaxMerit {
		t.Fatal("empty paragraph", k, err)
	}
}

func TestStreamingOwnerRetainsArenaAndFeatureSnapshot(t *testing.T) {
	cfg := StreamingConfig{Resolver: helpers.DefaultStyleResolver{}, Defaults: preparationDefaults(),
		NewShaper: func() (contracts.StreamingShaper, error) { return helpers.NewSyntheticStreamingShaper(0) }}
	cfg.Defaults.Features = []contracts.ShapeFeature{{Start: 1, End: 2}}
	owner := NewKhipukamayuq(Config{Streaming: &cfg})
	cfg.Defaults.Features[0].End = 99
	if owner.env.streaming.Defaults.Features[0].End != 2 {
		t.Fatal("feature alias")
	}
	// Synthetic shaping does not support features; use a separate configuration.
	cfg.Defaults.Features = nil
	owner = NewKhipukamayuq(Config{Streaming: &cfg})
	p := preparationParagraph(t, "a b")
	k, err := owner.EncodeParagraph(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if k.owner != owner || k.paragraph != p || len(k.shaping.boxes) != 2 || len(k.shaping.glyphs) != 2 || len(k.boxRefs) != 4 {
		t.Fatal("publication lost associations")
	}
}
