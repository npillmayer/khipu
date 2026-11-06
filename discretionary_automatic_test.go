package khipu

import (
	"errors"
	"reflect"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/helpers"
	"github.com/npillmayer/uax/bidi"
	"golang.org/x/text/language"
)

func automaticTestOwner(policy Hyphenator, factory func() (contracts.StreamingShaper, error)) *Khipukamayuq {
	if factory == nil {
		factory = func() (contracts.StreamingShaper, error) { return helpers.NewSyntheticStreamingShaper(0) }
	}
	return NewKhipukamayuq(Config{Hyphenator: policy, Streaming: &StreamingConfig{
		Resolver: preparationResolver(func(_ styled.Style, d contracts.ResolvedRunStyle) (contracts.ResolvedRunStyle, error) { return d, nil }),
		Defaults: preparationDefaults(), NewShaper: factory,
	}})
}

func TestAutomaticProviderRetainsChangedSpellingAndCache(t *testing.T) {
	calls := 0
	owner := automaticTestOwner(explicitMaterialFunc(func(word string, p *Params) ([]HyphenationFragment, error) {
		calls++
		if word != "éclair" || p.Language != language.French {
			t.Fatalf("wrong input/context: %q %v", word, p.Language)
		}
		return []HyphenationFragment{{PreBreak: "EC-", PostBreak: "lair", Penalty: 27, SourceSplit: SourceSplit{3, true}}}, nil
	}), nil)
	owner.env.streaming.Defaults.Language = language.French
	k, err := owner.EncodeParagraph(t.Context(), preparationParagraph(t, "xx éclair"))
	if err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatal("eager discovery")
	}
	cands, err := owner.DiscretionaryCandidates(k, 2)
	if err != nil || len(cands) != 1 {
		t.Fatal(cands, err)
	}
	c := cands[0]
	pre, post := k.shaping.boxes[c.preBox-1], k.shaping.boxes[c.postBox-1]
	if c.SourceSplit != (SourceSplit{6, true}) || c.PreBreak.Pos != 3 || c.PreBreak.Len != 3 || c.PostBreak.Pos != 6 || c.PreBreak.Penalty != 27 ||
		pre.replacement.text != "EC-" || post.replacement.text != "lair" {
		t.Fatal("lost source/replacement distinction", c, pre, post)
	}
	again, err := owner.DiscretionaryCandidates(k, 2)
	if err != nil || calls != 1 || !reflect.DeepEqual(cands, again) {
		t.Fatal("cache miss", calls, err)
	}
}

func TestAutomaticPreparationRejectsMalformedBatch(t *testing.T) {
	for _, mode := range []string{"unknown", "start", "end", "outside", "inside rune", "UTF8", "recursive", "duplicate"} {
		t.Run(mode, func(t *testing.T) {
			valid := HyphenationFragment{Variant: 1, PreBreak: "é-", PostBreak: "clair", SourceSplit: SourceSplit{2, true}}
			bad := valid
			bad.Variant = 2
			switch mode {
			case "unknown":
				bad.SourceSplit.Valid = false
			case "start":
				bad.SourceSplit.ByteOffset = 0
			case "end":
				bad.SourceSplit.ByteOffset = 7
			case "outside":
				bad.SourceSplit.ByteOffset = 8
			case "inside rune":
				bad.SourceSplit.ByteOffset = 1
			case "UTF8":
				bad.PostBreak = "\xff"
			case "recursive":
				bad.PreBreak = "é\u00ad"
			case "duplicate":
				bad.Variant = 1
			}
			streams := 0
			owner := automaticTestOwner(&testhyphenator{want: []HyphenationFragment{valid, bad}}, func() (contracts.StreamingShaper, error) { streams++; return helpers.NewSyntheticStreamingShaper(0) })
			k, err := owner.EncodeParagraph(t.Context(), preparationParagraph(t, "éclair"))
			if err != nil {
				t.Fatal(err)
			}
			before := k.shaping
			got, err := owner.DiscretionaryCandidates(k, 0)
			if got != nil || !errors.Is(err, contracts.ErrIllegalArguments) || streams != 1 || len(k.DiscretionariesAt(0)) != 0 || !reflect.DeepEqual(before, k.shaping) {
				t.Fatal("malformed response published/shaped", got, err, streams)
			}
		})
	}
}

func TestAutomaticVariantsAndEmptyFiltering(t *testing.T) {
	owner := automaticTestOwner(&testhyphenator{want: []HyphenationFragment{
		{PreBreak: "ab-", PostBreak: "cd", SourceSplit: SourceSplit{2, true}},
		{Variant: 1, PreBreak: "abc-", PostBreak: "d", SourceSplit: SourceSplit{3, true}},
		{PreBreak: "", PostBreak: "bcd", SourceSplit: SourceSplit{1, true}},
	}}, nil)
	k, err := owner.EncodeParagraph(t.Context(), preparationParagraph(t, "abcd"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := owner.DiscretionaryCandidates(k, 0)
	if err != nil || len(got) != 2 || got[0].Variant != 2 || got[1].Variant != 1 || got[0].PreBreak.Penalty != Penalty(owner.env.params.Hyphenpenalty) {
		t.Fatal(got, err)
	}
}

func TestAutomaticProviderUsesResolvedDirection(t *testing.T) {
	called := false
	owner := automaticTestOwner(explicitMaterialFunc(func(word string, p *Params) ([]HyphenationFragment, error) {
		called = true
		if word != "אבגד" || p.BidiDir != bidi.RightToLeft || p.Script != language.MustParseScript("Hebr") || p.Language != language.Hebrew {
			t.Fatal(word, p)
		}
		return nil, nil
	}), nil)
	owner.env.streaming.Defaults.Language = language.Hebrew
	owner.env.streaming.Defaults.Script = language.MustParseScript("Hebr")
	original := *owner.env.params
	k, err := owner.EncodeParagraph(t.Context(), preparationParagraph(t, "אבגד"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = owner.DiscretionaryCandidates(k, 0); err != nil || !called {
		t.Fatal(called, err)
	}
	if !reflect.DeepEqual(original, *owner.env.params) {
		t.Fatal("mutated owner defaults")
	}
}

func TestAutomaticProviderDoesNotQueryWordFragments(t *testing.T) {
	owner := automaticTestOwner(explicitMaterialFunc(func(string, *Params) ([]HyphenationFragment, error) { t.Fatal("queried partial word"); return nil, nil }), nil)
	for _, text := range []string{"metafinanz", "abcאבג", "ab\u2066cd\u2069ef"} {
		p := preparationParagraph(t, text)
		if text == "metafinanz" {
			if _, err := p.Style(intersectionStyle("logo"), 0, 4); err != nil {
				t.Fatal(err)
			}
		}
		k, err := owner.EncodeParagraph(t.Context(), p)
		if err != nil {
			t.Fatal(err)
		}
		for i := range k.Kind {
			if got, err := owner.DiscretionaryCandidates(k, i); err != nil || len(got) != 0 {
				t.Fatal(got, err)
			}
		}
	}
}

func TestAutomaticProviderBatchFailureAndRetry(t *testing.T) {
	failure := errors.New("automatic fragment factory failed")
	streams := 0
	policy := &testhyphenator{want: []HyphenationFragment{
		{PreBreak: "ab-", PostBreak: "cdef", SourceSplit: SourceSplit{2, true}},
		{PreBreak: "abcd-", PostBreak: "ef", SourceSplit: SourceSplit{4, true}},
	}}
	owner := automaticTestOwner(policy, func() (contracts.StreamingShaper, error) {
		streams++
		if streams == 4 {
			return nil, failure
		}
		return helpers.NewSyntheticStreamingShaper(0)
	})
	k, err := owner.EncodeParagraph(t.Context(), preparationParagraph(t, "abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	before := k.shaping
	got, err := owner.DiscretionaryCandidates(k, 0)
	if !errors.Is(err, failure) || got != nil || len(k.DiscretionariesAt(0)) != 0 || !reflect.DeepEqual(before, k.shaping) {
		t.Fatal("partial batch", got, err)
	}
	got, err = owner.DiscretionaryCandidates(k, 0)
	if err != nil || len(got) != 2 || policy.calls != 2 {
		t.Fatal("retry failed", got, err)
	}
}

func TestAutomaticAndExplicitRequestsShareParagraph(t *testing.T) {
	var requests []string
	owner := automaticTestOwner(explicitMaterialFunc(func(word string, _ *Params) ([]HyphenationFragment, error) {
		requests = append(requests, word)
		if word == softHyphenMarker {
			return []HyphenationFragment{{PreBreak: "-"}}, nil
		}
		return []HyphenationFragment{{PreBreak: "ab-", PostBreak: "cd", SourceSplit: SourceSplit{2, true}}}, nil
	}), nil)
	k, err := owner.EncodeParagraph(t.Context(), preparationParagraph(t, "abcd ab\u00adcd"))
	if err != nil {
		t.Fatal(err)
	}
	for _, at := range []int{0, 2} {
		got, err := owner.DiscretionaryCandidates(k, at)
		if err != nil || len(got) != 1 || got[0].preBox == 0 || got[0].postBox == 0 {
			t.Fatal(got, err)
		}
	}
	if !reflect.DeepEqual(requests, []string{"abcd", softHyphenMarker}) {
		t.Fatal("mixed discovery routes", requests)
	}
	if k.DiscretionariesAt(0)[0].preBox == k.DiscretionariesAt(2)[0].preBox {
		t.Fatal("shared fragment identity across boxes")
	}
}
