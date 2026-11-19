package khipu

import (
	"errors"
	"reflect"
	"testing"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/helpers"
)

func TestDiscretionaryEmptyResultsAreCached(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		fragments  []HyphenationFragment
	}{
		{"nil automatic", "abcd", nil},
		{"empty automatic", "abcd", []HyphenationFragment{}},
		{"filtered automatic", "abcd", []HyphenationFragment{{PostBreak: "bcd", SourceSplit: SourceSplit{1, true}}}},
		{"declined explicit", "ab\u00adcd", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			policy := &testhyphenator{want: tc.fragments}
			streams := 0
			owner := automaticTestOwner(policy, func() (contracts.StreamingShaper, error) {
				streams++
				return helpers.NewSyntheticStreamingShaper(0)
			})
			k, err := owner.EncodeParagraphFixture(preparationParagraph(t, tc.text))
			if err != nil {
				t.Fatal(err)
			}
			before := k.shaping
			if len(k.discovery.batches) != 0 || policy.calls != 0 {
				t.Fatal("encoding performed eager discovery")
			}
			for range 3 {
				if got, err := owner.DiscoverFixture(k, 0); err != nil || len(got) != 0 {
					t.Fatal(got, err)
				}
			}
			if _, cached := k.discovery.batches[0]; !cached || policy.calls != 1 || streams != 1 {
				t.Fatalf("cached=%v policy=%d streams=%d", cached, policy.calls, streams)
			}
			if len(k.DiscretionaryCandidates) != 0 || len(k.SelectedDiscretionaries) != 0 || !reflect.DeepEqual(before, k.shaping) {
				t.Fatal("empty discovery published candidates, selections or shaped fragments")
			}
			empty, _ := k.discovery.Alternatives(0)
			if len(empty.Candidates()) != 0 || len(empty.IDs()) != 0 {
				t.Fatal("empty set changed")
			}

		})
	}
}

func TestDiscretionaryEmptyCacheOwnership(t *testing.T) {
	policy := &testhyphenator{}
	owner := NewKhipukamayuq(Config{Hyphenator: policy})
	for paragraph := range 2 {
		k, err := owner.EncodeParagraphFixture(preparationParagraph(t, "abcd abcd"))
		if err != nil {
			t.Fatal(err)
		}
		for _, at := range []int{0, 2, 0, 2} {
			if got, err := owner.DiscoverFixture(k, at); err != nil || len(got) != 0 {
				t.Fatal(got, err)
			}
		}
		if policy.calls != (paragraph+1)*2 || len(k.discovery.batches) != 2 {
			t.Fatal("cache leaked between knots or paragraphs", policy.calls)
		}
		foreignPolicy := &testhyphenator{}
		foreign := NewKhipukamayuq(Config{Hyphenator: foreignPolicy})
		if _, err := foreign.DiscoverFixture(k, 0); !errors.Is(err, ErrIllegalArguments) || foreignPolicy.calls != 0 {
			t.Fatal("cached empty bypassed owner validation", err)
		}
		unwireKhipu(k)
		if k.discovery != nil {
			t.Fatal("unwire retained empty cache")
		}
	}
}

func TestDiscretionaryErrorsAreNotCached(t *testing.T) {
	for _, mode := range []string{"policy", "validation", "shaping"} {
		t.Run(mode, func(t *testing.T) {
			failure := errors.New("transient failure")
			fail, calls, streams := true, 0, 0
			owner := automaticTestOwner(explicitMaterialFunc(func(string, *Params) ([]HyphenationFragment, error) {
				calls++
				if fail && mode == "policy" {
					return nil, failure
				}
				if fail && mode == "validation" {
					return []HyphenationFragment{{PreBreak: "ab-", PostBreak: "cd"}}, nil
				}
				if fail { // valid response, but shaping fails
					return []HyphenationFragment{{PreBreak: "ab-", PostBreak: "cd", SourceSplit: SourceSplit{2, true}}}, nil
				}
				return nil, nil
			}), func() (contracts.StreamingShaper, error) {
				streams++
				if fail && mode == "shaping" && streams > 1 {
					return nil, failure
				}
				return helpers.NewSyntheticStreamingShaper(0)
			})
			k, err := owner.EncodeParagraphFixture(preparationParagraph(t, "abcd"))
			if err != nil {
				t.Fatal(err)
			}
			before := k.shaping
			for range 2 {
				_, err = owner.DiscoverFixture(k, 0)
				want := failure
				if mode == "validation" {
					want = ErrIllegalArguments
				}
				if !errors.Is(err, want) || len(k.discovery.batches) != 0 || len(k.DiscretionaryCandidates) != 0 || !reflect.DeepEqual(before, k.shaping) {
					t.Fatal("failure changed cache/arena", err)
				}
			}
			fail = false
			for range 2 {
				if got, err := owner.DiscoverFixture(k, 0); err != nil || len(got) != 0 {
					t.Fatal("retry did not recover", got, err)
				}
			}
			if calls != 3 || len(k.discovery.batches) != 1 {
				t.Fatal("errors cached or successful retry not cached", calls)
			}
		})
	}
}

func TestDiscretionaryEmptyCacheExcludesDisabledAndManualRequests(t *testing.T) {
	owner := NewKhipukamayuq(Config{})
	k, err := owner.EncodeParagraphFixture(preparationParagraph(t, "abcd"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = owner.DiscoverFixture(k, 0); err != nil || len(k.DiscretionaryCandidates) != 0 {
		t.Fatal("disabled discovery returned candidates", err)
	}
	owner = NewKhipukamayuq(Config{Hyphenator: &testhyphenator{}})
	manual := &FixtureKhipu{Kind: []KnotType{KTTextBox, KTGlue}}
	for _, at := range []int{0, 1} {
		if _, err = owner.DiscoverFixture(manual, at); err != nil || manual.discretionaryEmpty != nil {
			t.Fatal("ownerless/manual lookup populated cache", err)
		}
	}
	for _, at := range []int{-1, 2} {
		if _, err = owner.DiscoverFixture(manual, at); !errors.Is(err, ErrIllegalArguments) || manual.discretionaryEmpty != nil {
			t.Fatal("invalid index populated cache", err)
		}
	}
}
