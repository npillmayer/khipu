package khipu

import (
	"errors"
	"reflect"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
	"github.com/npillmayer/uax/bidi"
	"golang.org/x/text/language"
	textbidi "golang.org/x/text/unicode/bidi"
)

type preparationResolver func(styled.Style, contracts.ResolvedRunStyle) (contracts.ResolvedRunStyle, error)

func (f preparationResolver) ResolveStyle(s styled.Style, d contracts.ResolvedRunStyle) (contracts.ResolvedRunStyle, error) {
	return f(s, d)
}

func preparationParagraph(t *testing.T, source string) *styled.Paragraph {
	t.Helper()
	text := styled.TextFromString(source)
	p, err := styled.ParagraphFromText(&text, 0, uint64(len(source)), bidi.LeftToRight, nil)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func preparationDefaults() contracts.ResolvedRunStyle {
	return contracts.ResolvedRunStyle{Font: helpers.NewSyntheticFont(), EmSize: 10 * dimen.PT}
}

func TestPrepareParagraphRuns(t *testing.T) {
	p := preparationParagraph(t, "a\u2067אב\u2069z")
	d := preparationDefaults()
	d.Language = language.German
	d.Spacing = contracts.SpacingElastic
	d.Features = []contracts.ShapeFeature{{Start: 1, End: 5, Enabled: true}}
	calls := 0
	resolver := preparationResolver(func(_ styled.Style, in contracts.ResolvedRunStyle) (contracts.ResolvedRunStyle, error) {
		calls++
		return in, nil
	})
	runs, err := prepareParagraphRuns(p, resolver, d)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || len(runs) != 3 {
		t.Fatalf("calls=%d runs=%v", calls, runs)
	}
	if len(runs[0].params.shaping.Features) != 0 || len(runs[2].params.shaping.Features) != 0 {
		t.Fatal("feature escaped source range")
	}
	f := runs[1].params.shaping.Features
	if len(f) != 1 || f[0].Start != 0 || f[0].End != 2 {
		t.Fatalf("feature = %v", f)
	}
	if runs[1].source.scalarStart != 2 || runs[1].params.shaping.Direction != textbidi.RightToLeft {
		t.Fatal("lost gap or direction")
	}
	for _, r := range runs {
		if r.params.scale.ToDU(500) != float64(5*dimen.PT) || r.params.emSize != d.EmSize || r.params.spacing != d.Spacing || r.params.shaping.Language != d.Language || r.params.shaping.Font != d.Font {
			t.Fatalf("parameters %+v", r.params)
		}
	}
	d.Features[0].Enabled = false
	if !f[0].Enabled {
		t.Fatal("features alias defaults")
	}
}

func TestPreparationIndependentStyles(t *testing.T) {
	p := preparationParagraph(t, "abc")
	if _, err := p.Style(intersectionStyle("large"), 0, 1); err != nil {
		t.Fatal(err)
	}
	d := preparationDefaults()
	calls := 0
	r := preparationResolver(func(s styled.Style, in contracts.ResolvedRunStyle) (contracts.ResolvedRunStyle, error) {
		calls++
		if in.EmSize != d.EmSize {
			t.Fatal("inherited preceding style")
		}
		if s != nil {
			in.EmSize = 20 * dimen.PT
		}
		return in, nil
	})
	runs, err := prepareParagraphRuns(p, r, d)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(runs) != 2 || runs[0].params.scale.ToDU(500) != float64(10*dimen.PT) || runs[1].params.scale.ToDU(500) != float64(5*dimen.PT) {
		t.Fatal("incorrect span resolution", runs)
	}
}

func TestPreparationHelpersAndOwnership(t *testing.T) {
	d := preparationDefaults()
	d.Features = []contracts.ShapeFeature{{Enabled: true}}
	runs, err := prepareParagraphRuns(preparationParagraph(t, "aאבz"), helpers.DefaultStyleResolver{}, d)
	if err != nil {
		t.Fatal(err)
	}
	runs[0].params.shaping.Features[0].Enabled = false
	if !runs[1].params.shaping.Features[0].Enabled || !d.Features[0].Enabled {
		t.Fatal("shared feature storage")
	}
	if runs[0].params.shaping.Script != d.Script || runs[0].params.spacing != contracts.SpacingFixed {
		t.Fatal("changed explicit defaults")
	}
	for _, s := range []string{"", "\u2066\u2069"} {
		got, err := prepareParagraphRuns(preparationParagraph(t, s), helpers.DefaultStyleResolver{}, d)
		if err != nil || len(got) != 0 {
			t.Fatalf("empty input %q: %v %v", s, got, err)
		}
	}
}

func TestIntersectFeatures(t *testing.T) {
	for _, tc := range []struct {
		name       string
		start, end int
		want       []contracts.ShapeFeature
		bad        bool
	}{
		{"crosses", 4, 10, []contracts.ShapeFeature{{Start: 0, End: 4}}, false},
		{"whole", 0, 0, []contracts.ShapeFeature{{Start: 0, End: 6}}, false},
		{"negative sentinels", -1, -1, []contracts.ShapeFeature{{Start: 0, End: 6}}, false},
		{"touches", 0, 6, nil, false},
		{"empty", 8, 8, nil, false},
		{"outside", 20, 30, nil, false},
		{"clipped", 8, 30, []contracts.ShapeFeature{{Start: 2, End: 6}}, false},
		{"reversed", 10, 4, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := intersectFeatures([]contracts.ShapeFeature{{Start: tc.start, End: tc.end}}, 12, 6, 12)
			if tc.bad {
				if !errors.Is(err, contracts.ErrIllegalArguments) {
					t.Fatal(err)
				}
				return
			}
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v %v, want %v", got, err, tc.want)
			}
		})
	}
}

type preparationFont struct {
	helpers.SyntheticFont
	err      error
	nilScale bool
}

func (f preparationFont) Scale(size dimen.DU) (contracts.FontScale, error) {
	if f.err != nil || f.nilScale {
		return nil, f.err
	}
	return f.SyntheticFont.Scale(size)
}

func TestPreparationFailures(t *testing.T) {
	p := preparationParagraph(t, "abc")
	failure := errors.New("backend failure")
	for _, tc := range []struct {
		name     string
		resolver contracts.StyleResolver
		defaults contracts.ResolvedRunStyle
		want     error
	}{
		{"nil resolver", nil, preparationDefaults(), contracts.ErrIllegalArguments},
		{"resolver error", preparationResolver(func(styled.Style, contracts.ResolvedRunStyle) (contracts.ResolvedRunStyle, error) {
			return contracts.ResolvedRunStyle{}, failure
		}), preparationDefaults(), failure},
		{"nil font", helpers.DefaultStyleResolver{}, contracts.ResolvedRunStyle{EmSize: dimen.PT}, contracts.ErrIllegalArguments},
		{"scale error", helpers.DefaultStyleResolver{}, contracts.ResolvedRunStyle{Font: preparationFont{err: failure}, EmSize: dimen.PT}, failure},
		{"nil scale", helpers.DefaultStyleResolver{}, contracts.ResolvedRunStyle{Font: preparationFont{nilScale: true}, EmSize: dimen.PT}, contracts.ErrIllegalArguments},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := prepareParagraphRuns(p, tc.resolver, tc.defaults)
			if got != nil || !errors.Is(err, tc.want) {
				t.Fatalf("got %v %v", got, err)
			}
		})
	}
}

func TestPreparationFailureDiscardsEarlierRuns(t *testing.T) {
	p := preparationParagraph(t, "abc")
	if _, err := p.Style(intersectionStyle("bad"), 1, 3); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("later style failure")
	r := preparationResolver(func(s styled.Style, d contracts.ResolvedRunStyle) (contracts.ResolvedRunStyle, error) {
		if s != nil {
			return contracts.ResolvedRunStyle{}, failure
		}
		return d, nil
	})
	got, err := prepareParagraphRuns(p, r, preparationDefaults())
	if got != nil || !errors.Is(err, failure) {
		t.Fatalf("partial result: %v %v", got, err)
	}
}

func TestPreparationFeatureOriginIsStyleSpan(t *testing.T) {
	p := preparationParagraph(t, "x\u2067אב\u2069z")
	// This style begins at the isolate control, not at its first shaped scalar.
	if _, err := p.Style(intersectionStyle("range"), 1, 11); err != nil {
		t.Fatal(err)
	}
	r := preparationResolver(func(s styled.Style, d contracts.ResolvedRunStyle) (contracts.ResolvedRunStyle, error) {
		if s != nil {
			d.Features = []contracts.ShapeFeature{{Start: 2, End: 3}}
		}
		return d, nil
	})
	got, err := prepareParagraphRuns(p, r, preparationDefaults())
	if err != nil {
		t.Fatal(err)
	}
	f := got[1].params.shaping.Features
	if len(f) != 1 || f[0].Start != 1 || f[0].End != 2 {
		t.Fatalf("wrong style-relative feature: %v", f)
	}
}
