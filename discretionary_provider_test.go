package khipu

import (
	"errors"
	"reflect"
	"testing"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/helpers"
)

func TestExplicitProviderBatchAndRetry(t *testing.T) {
	failure := errors.New("later fragment failed")
	calls := 0
	fail := true
	policy := &testhyphenator{want: []HyphenationFragment{{PreBreak: "-"}}}
	owner := NewKhipukamayuq(Config{Hyphenator: policy, Streaming: &StreamingConfig{
		Resolver: helpers.DefaultStyleResolver{}, Defaults: preparationDefaults(),
		NewShaper: func() (contracts.StreamingShaper, error) {
			calls++
			if fail && calls == 4 {
				return nil, failure
			}
			return helpers.NewSyntheticStreamingShaper(0)
		},
	}})
	k, err := owner.EncodeParagraphStreaming(t.Context(), preparationParagraph(t, "ab\u00adcd\u00adef"))
	if err != nil {
		t.Fatal(err)
	}
	before := k.shaping
	got, err := owner.DiscretionaryCandidates(k, 0)
	if got != nil || !errors.Is(err, failure) {
		t.Fatal(got, err)
	}
	if !reflect.DeepEqual(before, k.shaping) || len(k.DiscretionariesAt(0)) != 0 {
		t.Fatal("partial batch published")
	}
	fail = false
	got, err = owner.DiscretionaryCandidates(k, 0)
	if err != nil || len(got) != 2 {
		t.Fatal(got, err)
	}
	for i, cand := range got {
		if cand.preBox == 0 || cand.postBox == 0 {
			t.Fatal("missing retained fragments")
		}
		pre := k.shaping.boxes[cand.preBox-1]
		post := k.shaping.boxes[cand.postBox-1]
		want := [][2]string{{"ab-", "cdef"}, {"abcd-", "ef"}}[i]
		if pre.replacement.text != want[0] || post.replacement.text != want[1] {
			t.Fatal("incorrect stored input")
		}
		if k.shaping.contexts[pre.context].params.shaping.Font != owner.env.streaming.Defaults.Font {
			t.Fatal("lost retained font")
		}
	}
	streamCalls, policyCalls := calls, policy.calls
	again, err := owner.DiscretionaryCandidates(k, 0)
	if err != nil || !reflect.DeepEqual(again, got) || calls != streamCalls || policy.calls != policyCalls {
		t.Fatal("cache miss")
	}
}

func TestExplicitProviderRejectsInteriorFeatures(t *testing.T) {
	calls := 0
	owner := NewKhipukamayuq(Config{Hyphenator: &testhyphenator{want: []HyphenationFragment{{PreBreak: "-"}}}, Streaming: &StreamingConfig{
		Resolver: helpers.DefaultStyleResolver{}, Defaults: preparationDefaults(),
		NewShaper: func() (contracts.StreamingShaper, error) { calls++; return helpers.NewSyntheticStreamingShaper(0) },
	}})
	k, err := owner.EncodeParagraphStreaming(t.Context(), preparationParagraph(t, "ab\u00adcd"))
	if err != nil {
		t.Fatal(err)
	}
	// Seed metadata after synthetic encoding; the synthetic backend itself
	// deliberately rejects all feature settings.
	k.shaping.contexts[0].params.shaping.Features = []contracts.ShapeFeature{{Start: 1, End: 2}}
	before := k.shaping
	got, err := owner.DiscretionaryCandidates(k, 0)
	if got != nil || !errors.Is(err, contracts.ErrIllegalArguments) || calls != 1 || !reflect.DeepEqual(before, k.shaping) {
		t.Fatal(got, err, calls)
	}
}

func TestExplicitProviderDeclinesMarkerEdges(t *testing.T) {
	policy := &testhyphenator{want: []HyphenationFragment{{PreBreak: "-"}}}
	owner := NewKhipukamayuq(Config{Hyphenator: policy, Streaming: &StreamingConfig{
		Resolver: helpers.DefaultStyleResolver{}, Defaults: preparationDefaults(),
		NewShaper: func() (contracts.StreamingShaper, error) { return helpers.NewSyntheticStreamingShaper(0) },
	}})
	for _, text := range []string{"abc", "ab\u00ad", "\u00adcd"} {
		k, err := owner.EncodeParagraphStreaming(t.Context(), preparationParagraph(t, text))
		if err != nil {
			t.Fatal(err)
		}
		got, err := owner.DiscretionaryCandidates(k, 0)
		if err != nil || len(got) != 0 {
			t.Fatal(text, got, err)
		}
	}
	if policy.calls != 0 {
		t.Fatal("consulted automatic policy or incomplete source")
	}
}

func TestExplicitProviderNormalizesWholeBoxFeatures(t *testing.T) {
	owner := NewKhipukamayuq(Config{Hyphenator: &testhyphenator{want: []HyphenationFragment{{PreBreak: "-"}}}, Streaming: &StreamingConfig{
		Resolver: helpers.DefaultStyleResolver{}, Defaults: preparationDefaults(),
		NewShaper: func() (contracts.StreamingShaper, error) { return helpers.NewSyntheticStreamingShaper(0) },
	}})
	k, err := owner.EncodeParagraphStreaming(t.Context(), preparationParagraph(t, "ab\u00adcd"))
	if err != nil {
		t.Fatal(err)
	}
	original := contracts.ShapeFeature{Enabled: true, Start: 0, End: 5}
	k.shaping.contexts[0].params.shaping.Features = []contracts.ShapeFeature{original}
	var streams []*fragmentRecordingShaper
	owner.env.streaming.NewShaper = func() (contracts.StreamingShaper, error) {
		s := &fragmentRecordingShaper{pipelineShaper: newPipelineShaper()}
		streams = append(streams, s)
		return s, nil
	}
	got, err := owner.DiscretionaryCandidates(k, 0)
	if err != nil || len(got) != 1 || len(streams) != 2 {
		t.Fatal(got, err)
	}
	for _, s := range streams {
		if len(s.params.Features) != 1 || !s.params.Features[0].Enabled || s.params.Features[0].Start != 0 || s.params.Features[0].End != 0 {
			t.Fatal("whole-box feature not mapped", s.params.Features)
		}
	}
	if k.shaping.contexts[0].params.shaping.Features[0] != original {
		t.Fatal("original context mutated")
	}
}
