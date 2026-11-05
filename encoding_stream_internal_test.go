package khipu

import (
	"context"
	"testing"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/helpers"
)

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
	k, err := owner.EncodeParagraphStreaming(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if k.owner != owner || k.paragraph != p || len(k.shaping.boxes) != 2 || len(k.shaping.glyphs) != 2 || len(k.boxRefs) != 4 {
		t.Fatal("publication lost associations")
	}
}
