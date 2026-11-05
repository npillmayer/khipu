package khipu

import (
	"context"
	"errors"
	"testing"
	"unicode/utf8"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/helpers"
)

type explicitMaterialFunc func(string, *Params) ([]HyphenationFragment, error)

func (f explicitMaterialFunc) Discretionaries(s string, p *Params) ([]HyphenationFragment, error) {
	return f(s, p)
}

func explicitSource(s string) runSourceRange {
	return runSourceRange{10, 10 + uint64(len(s)), 20, 20 + uint64(utf8.RuneCountInString(s))}
}

func TestPrepareExplicitDiscretionaries(t *testing.T) {
	calls := 0
	owner := NewKhipukamayuq(Config{Hyphenator: explicitMaterialFunc(func(s string, p *Params) ([]HyphenationFragment, error) {
		calls++
		if s != softHyphenMarker {
			t.Fatalf("requested automatic discovery for %q", s)
		}
		return []HyphenationFragment{{PreBreak: "-", Penalty: 31}, {PreBreak: "~", PostBreak: "!", Penalty: 42}}, nil
	})})
	text := "co\u00adop\u00aderate"
	got, err := owner.prepareExplicitDiscretionaries(text, explicitSource(text))
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || len(got) != 4 {
		t.Fatalf("calls=%d variants=%d", calls, len(got))
	}
	for i, want := range [][2]string{{"co-", "operate"}, {"co~", "!operate"}, {"coop-", "erate"}, {"coop~", "!erate"}} {
		if got[i].input.pre.text != want[0] || got[i].input.post.text != want[1] || got[i].variant != uint16(i+1) {
			t.Fatal(got[i])
		}
	}
	if got[0].input.split != (SourceSplit{14, true}) || got[2].input.split != (SourceSplit{18, true}) || got[0].penalty != 31 || got[1].penalty != 42 {
		t.Fatal("wrong original positions or policy penalties", got)
	}
	if got[0].input.pre.provenance.scalarEnd != 23 {
		t.Fatal("marker omitted from original provenance")
	}
}

func TestExplicitPreparationDeclinesAndErrors(t *testing.T) {
	failure := errors.New("policy failure")
	for _, mode := range []string{"no marker", "endpoints", "disabled", "declined", "failure", "UTF8", "recursive", "source"} {
		t.Run(mode, func(t *testing.T) {
			text := "a\u00adb"
			var want error
			calls := 0
			policy := explicitMaterialFunc(func(string, *Params) ([]HyphenationFragment, error) {
				calls++
				switch mode {
				case "declined":
					return nil, nil
				case "failure":
					return nil, failure
				case "UTF8":
					return []HyphenationFragment{{PreBreak: "-"}, {PostBreak: "\xff"}}, nil
				case "recursive":
					return []HyphenationFragment{{PreBreak: softHyphenMarker}}, nil
				}
				return []HyphenationFragment{{PreBreak: "-"}}, nil
			})
			cfg := Config{Hyphenator: policy}
			switch mode {
			case "no marker":
				text = "abc"
			case "endpoints":
				text = "\u00adabc\u00ad"
			case "disabled":
				cfg.Hyphenator = nil
			case "failure":
				want = failure
			case "UTF8", "recursive", "source":
				want = contracts.ErrIllegalArguments
			}
			source := explicitSource(text)
			if mode == "source" {
				source.byteEnd++
			}
			got, err := NewKhipukamayuq(cfg).prepareExplicitDiscretionaries(text, source)
			if got != nil || !errors.Is(err, want) {
				t.Fatalf("%+v %v", got, err)
			}
			if (mode == "no marker" || mode == "endpoints" || mode == "disabled" || mode == "source") && calls != 0 {
				t.Fatal("unneeded policy call")
			}
		})
	}
}

func TestExplicitPreparationUsesCommonShaping(t *testing.T) {
	owner := NewKhipukamayuq(Config{Hyphenator: explicitMaterialFunc(func(string, *Params) ([]HyphenationFragment, error) {
		return []HyphenationFragment{{PreBreak: "-"}}, nil
	})})
	text := "co\u00adoperate"
	prepared, err := owner.prepareExplicitDiscretionaries(text, explicitSource(text))
	if err != nil {
		t.Fatal(err)
	}
	_, params := fragmentFixture(t)
	var streams []*fragmentRecordingShaper
	pair, err := shapeDiscretionaryInput(context.Background(), prepared[0].input, params, params, func() (contracts.StreamingShaper, error) {
		s := &fragmentRecordingShaper{pipelineShaper: newPipelineShaper()}
		streams = append(streams, s)
		return s, nil
	}, contracts.ShapeBufferOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if streams[0].text != "co-" || streams[1].text != "operate" || pair.split.ByteOffset != 14 {
		t.Fatal("not shaping whole fragments")
	}
	if prepared[0].penalty != Penalty(owner.env.params.Hyphenpenalty) {
		t.Fatal("lost owner penalty fallback")
	}
}

func TestExplicitEmptyMaterialAndMultibyteInput(t *testing.T) {
	owner := NewKhipukamayuq(Config{Hyphenator: &testhyphenator{want: []HyphenationFragment{{}}}})
	text := "é\u00adz"
	got, err := owner.prepareExplicitDiscretionaries(text, explicitSource(text))
	if err != nil || len(got) != 1 {
		t.Fatal(got, err)
	}
	if got[0].input.pre.text != "é" || got[0].input.post.text != "z" || got[0].input.split.ByteOffset != 14 {
		t.Fatal(got[0])
	}
	_, params := fragmentFixture(t)
	if _, err := shapeDiscretionaryInput(context.Background(), got[0].input, params, params, func() (contracts.StreamingShaper, error) {
		return helpers.NewSyntheticStreamingShaper(0)
	}, contracts.ShapeBufferOptions{}); err != nil {
		t.Fatal(err)
	}
}
