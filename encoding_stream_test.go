package khipu_test

import (
	"context"
	"errors"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/khipu/linebreak/firstfit"
	"github.com/npillmayer/khipu/linebreak/knuthplass"
	"github.com/npillmayer/uax/bidi"
)

func streamingParagraph(t *testing.T) *styled.Paragraph {
	t.Helper()
	text := styled.TextFromString("Hello world")
	p, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), bidi.LeftToRight, nil)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPublicStreamingEncodingAndBreakers(t *testing.T) {
	calls := 0
	cfg := khipu.StreamingConfig{
		NewShaper: func() (contracts.StreamingShaper, error) { calls++; return helpers.NewSyntheticStreamingShaper(1) },
		Resolver:  helpers.DefaultStyleResolver{},
		Defaults:  contracts.ResolvedRunStyle{Font: helpers.NewSyntheticFont(), EmSize: 10 * dimen.PT, Spacing: contracts.SpacingElastic},
	}
	owner := khipu.NewKhipukamayuq(khipu.Config{Streaming: &cfg})
	cfg.Defaults.EmSize = 20 * dimen.PT // owner must retain its construction snapshot
	for i := 0; i < 2; i++ {
		khp, err := owner.EncodeParagraphStreaming(context.Background(), streamingParagraph(t))
		if err != nil {
			t.Fatal(err)
		}
		if len(khp.Kind) != 4 || khp.W[0] != 25*dimen.PT || khp.MinW[1] >= khp.W[1] || khp.MaxW[1] <= khp.W[1] {
			t.Fatal("unexpected metrics", khp)
		}
		shape := linebreak.RectangularParShape(100 * dimen.PT)
		greedy, err := firstfit.BreakParagraph(khp, shape, nil)
		if err != nil {
			t.Fatal(err)
		}
		optimal, err := knuthplass.BreakParagraph(khp, shape, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(greedy) != 1 || len(optimal) != 1 || greedy[0] != 3 || optimal[0] != 3 {
			t.Fatal(greedy, optimal)
		}
		if c, err := owner.DiscretionaryCandidates(khp, 0); err != nil || len(c) != 0 {
			t.Fatal("disabled hyphenation", c, err)
		}
	}
	if calls != 2 {
		t.Fatal("factory reuse", calls)
	}
	// Streaming configuration does not replace metric encoding.
	if _, err := owner.EncodeParagraph(streamingParagraph(t), nil); err != nil {
		t.Fatal(err)
	}
}

func TestPublicStreamingConfigurationErrors(t *testing.T) {
	p := streamingParagraph(t)
	failure := errors.New("factory failed")
	for _, cfg := range []*khipu.StreamingConfig{
		nil, {},
		{Resolver: helpers.DefaultStyleResolver{}, NewShaper: func() (contracts.StreamingShaper, error) { return nil, nil }},
		{Resolver: helpers.DefaultStyleResolver{}, NewShaper: func() (contracts.StreamingShaper, error) { return nil, failure }},
	} {
		owner := khipu.NewKhipukamayuq(khipu.Config{Streaming: cfg})
		got, err := owner.EncodeParagraphStreaming(context.Background(), p)
		if got != nil || err == nil {
			t.Fatal("accepted missing config")
		}
		if cfg != nil && cfg.NewShaper != nil {
			_, factoryErr := cfg.NewShaper()
			if factoryErr != nil && !errors.Is(err, failure) {
				t.Fatal("lost cause", err)
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := khipu.NewKhipukamayuq(khipu.Config{}).EncodeParagraphStreaming(ctx, p); got != nil || !errors.Is(err, context.Canceled) {
		t.Fatal(got, err)
	}
}

func TestStreamingAutomaticDiscretionariesRemainDisabled(t *testing.T) {
	owner := khipu.NewKhipukamayuq(khipu.Config{Hyphenator: exampleHyphenator{}, Streaming: &khipu.StreamingConfig{
		NewShaper: func() (contracts.StreamingShaper, error) { return helpers.NewSyntheticStreamingShaper(1) },
		Resolver:  helpers.DefaultStyleResolver{}, Defaults: contracts.ResolvedRunStyle{Font: helpers.NewSyntheticFont(), EmSize: 10 * dimen.PT},
	}})
	k, err := owner.EncodeParagraphStreaming(context.Background(), streamingParagraph(t))
	if err != nil {
		t.Fatal(err)
	}
	if candidates, err := owner.DiscretionaryCandidates(k, 0); err != nil || len(candidates) != 0 {
		t.Fatal("unexpected automatic streaming candidates", candidates, err)
	}
}

func TestPublicStreamingBoundaryRegressions(t *testing.T) {
	for _, tc := range []struct {
		text  string
		lines int
	}{
		{"a\nb", 2}, {"a\r\nb", 2}, {"a\n\nb", 3}, {"a אב z", 1}, {"a\u2067אב\u2069z", 1},
		{"a\u00a0b", 1}, {"a\u2003b", 1}, {"a\u200bb", 1}, {"e\u0301 z", 1},
	} {
		t.Run(tc.text, func(t *testing.T) {
			text := styled.TextFromString(tc.text)
			p, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), bidi.LeftToRight, nil)
			if err != nil {
				t.Fatal(err)
			}
			owner := khipu.NewKhipukamayuq(khipu.Config{Streaming: &khipu.StreamingConfig{
				Resolver: helpers.DefaultStyleResolver{}, Defaults: contracts.ResolvedRunStyle{Font: helpers.NewSyntheticFont(), EmSize: 10 * dimen.PT, Spacing: contracts.SpacingElastic},
				NewShaper: func() (contracts.StreamingShaper, error) { return helpers.NewSyntheticStreamingShaper(1) },
			}})
			k, err := owner.EncodeParagraphStreaming(t.Context(), p)
			if err != nil {
				t.Fatal(err)
			}
			shape := linebreak.RectangularParShape(100 * dimen.PT)
			f, err := firstfit.BreakParagraph(k, shape, nil)
			if err != nil {
				t.Fatal(err)
			}
			kp, err := knuthplass.BreakParagraph(k, shape, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(f) != tc.lines || len(kp) != tc.lines {
				t.Fatalf("lines firstfit=%v KP=%v", f, kp)
			}
			for i := range k.Kind {
				if k.MinW[i] > k.W[i] || k.W[i] > k.MaxW[i] {
					t.Fatal("invalid width bounds")
				}
			}
		})
	}
}
