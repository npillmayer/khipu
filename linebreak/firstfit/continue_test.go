package firstfit

import (
	"context"
	"errors"
	"maps"
	"runtime"
	"slices"
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/khipu/linebreak/knuthplass"
	"github.com/npillmayer/uax/bidi"
)

func TestContinueIndependentBreakpoints(t *testing.T) {
	q, p, k := continuationFixture(t, "aa bb cc dd ee ff gg hh ii jj kk ll", bidi.LeftToRight, false)
	layout := DefaultLayout(linebreak.RectangularParShape(40 * dimen.BP))
	path, err := BreakParagraph(k, layout, nil)
	if err != nil {
		t.Fatal(err)
	}
	positions, err := k.BreakpointPositions(path)
	if err != nil || !slices.Equal(positions, []uint64{9, 18, 27, 35}) {
		t.Fatal(positions, err)
	}
	original := checkContinuationFrozen(t, q, k, path, layout)
	c, err := q.Checkpoint(t.Context(), p, path, 1)
	if err != nil {
		t.Fatal(err)
	}
	prefix := slices.Clone(path[:1])
	for _, tc := range []struct {
		width dimen.DU
		want  []uint64
	}{
		{20, []uint64{9, 12, 15, 18, 21, 24, 27, 30, 33, 35}},
		{30, []uint64{9, 15, 21, 27, 33, 35}},
		{55, []uint64{9, 21, 33, 35}},
	} {
		fresh := continuationBind(t, q, p)
		shape := &continuationShape{prefix: []dimen.DU{40 * dimen.BP}, tail: tc.width * dimen.BP}
		layout.LineShape = shape
		got, err := ContinueParagraph(t.Context(), fresh, layout, c)
		if err != nil {
			t.Fatal(err)
		}
		for _, n := range shape.visits {
			if n <= 1 {
				t.Fatal("prefix searched", shape.visits)
			}
		}
		positions, err := fresh.BreakpointPositions(got)
		if err != nil || !slices.Equal(positions, tc.want) {
			t.Fatal(tc.width, positions, tc.want, err)
		}
		ref := continuationReference(t, fresh, layout, prefix)
		if !slices.Equal(got, ref) {
			t.Fatal("reference mismatch", got, ref)
		}
		frozen := checkContinuationFrozen(t, q, fresh, got, layout)
		checkContinuationPrefix(t, original, frozen, 1)
	}
}

func TestContinueDistinctKPPrefixAndThirdWidth(t *testing.T) {
	q, p, k := continuationFixture(t, "aaaa a a aa aa aaaa aa aaa a a aaa", bidi.LeftToRight, true)
	layout := knuthplass.DefaultLayout(linebreak.RectangularParShape(40 * dimen.BP))
	// Use identical explicit skips for both searches and all freezes.
	layout.LeftSkip, layout.RightSkip = khipu.KnotCore{}, khipu.KnotCore{}
	ff, err := BreakParagraph(k, layout, nil)
	if err != nil {
		t.Fatal(err)
	}
	path, err := knuthplass.BreakParagraph(k, layout, nil)
	if err != nil {
		t.Fatal(err)
	}
	ffpos, _ := k.BreakpointPositions(ff)
	kppos, _ := k.BreakpointPositions(path)
	if !slices.Equal(ffpos, []uint64{9, 15, 23, 31, 34}) || !slices.Equal(kppos, []uint64{9, 20, 29, 34}) {
		t.Fatal("distinct KP oracle changed", ffpos, kppos)
	}
	original := checkContinuationFrozen(t, q, k, path, layout)
	c, err := q.Checkpoint(t.Context(), p, path, 2)
	if err != nil {
		t.Fatal(err)
	}
	b := continuationBind(t, q, p)
	layout.LineShape = &continuationShape{prefix: []dimen.DU{40 * dimen.BP, 40 * dimen.BP}, tail: 30 * dimen.BP}
	continued, err := ContinueParagraph(t.Context(), b, layout, c)
	if err != nil || !slices.Equal(continued[:2], path[:2]) {
		t.Fatal(continued, err)
	}
	if !slices.Equal(continued, continuationReference(t, b, layout, path[:2])) {
		t.Fatal("KP constrained reference")
	}
	bf := checkContinuationFrozen(t, q, b, continued, layout)
	checkContinuationPrefix(t, original, bf, 2)
	next, err := q.Checkpoint(t.Context(), p, continued, 3)
	if err != nil {
		t.Fatal(err)
	}
	third := continuationBind(t, q, p)
	layout.LineShape = &continuationShape{prefix: []dimen.DU{40 * dimen.BP, 40 * dimen.BP, 30 * dimen.BP}, tail: 20 * dimen.BP}
	last, err := ContinueParagraph(t.Context(), third, layout, next)
	if err != nil || !slices.Equal(last[:3], continued[:3]) {
		t.Fatal(last, err)
	}
	checkContinuationPrefix(t, bf, checkContinuationFrozen(t, q, third, last, layout), 3)
	// The newer checkpoint cannot mutate an earlier one.
	old, _ := c.PrefixFor(t.Context(), third)
	if !slices.Equal(old, path[:2]) {
		t.Fatal(old, path)
	}
}

func TestContinueForcedTrimmingBidiAndEndPrefixes(t *testing.T) {
	for _, tc := range []struct {
		text string
		dir  bidi.Direction
	}{
		{"", bidi.LeftToRight}, {"   ", bidi.LeftToRight},
		{"  aa bb\n\ncc dd ee ff  ", bidi.LeftToRight},
		{"אב גד הו זח טי כל מנ ס", bidi.RightToLeft},
		{"abc \u2067אב 12 גד הו\u2069 (xyz) dd ee ff", bidi.LeftToRight},
	} {
		q, p, k := continuationFixture(t, tc.text, tc.dir, false)
		layout := DefaultLayout(linebreak.RectangularParShape(40 * dimen.BP))
		path, err := BreakParagraph(k, layout, nil)
		if err != nil {
			t.Fatal(err)
		}
		original := checkContinuationFrozen(t, q, k, path, layout)
		for count := 0; count <= len(path); count++ {
			c, err := q.Checkpoint(t.Context(), p, path, count)
			if err != nil {
				t.Fatal(err)
			}
			fresh := continuationBind(t, q, p)
			widths := make([]dimen.DU, count)
			for i := range widths {
				widths[i] = 40 * dimen.BP
			}
			shape := &continuationShape{prefix: widths, tail: 20 * dimen.BP}
			layout.LineShape = shape
			probe := &continueCancelContext{Context: t.Context()}
			got, err := ContinueParagraph(probe, fresh, layout, c)
			if err != nil {
				t.Fatal(err)
			}
			// Two PrefixFor checks, search entry/publication, one check per
			// visited knot, and one after each recorded width query. Count the
			// actual loop visits independently of which lines request widths.
			start := 0
			if count > 0 {
				start = path[count-1] + 1
			}
			if visits := probe.calls - 4 - len(shape.visits); visits != len(k.Kind)-start {
				t.Fatal("search revisited prefix knots", visits, len(k.Kind)-start)
			}
			for _, n := range shape.visits {
				if int(n) <= count {
					t.Fatal("prefix search", count, shape.visits)
				}
			}
			if !slices.Equal(got, continuationReference(t, fresh, layout, path[:count])) {
				t.Fatal("reference", tc.text, count)
			}
			checkContinuationPrefix(t, original, checkContinuationFrozen(t, q, fresh, got, layout), count)
		}
	}
}

func TestContinueFrozenLifetime(t *testing.T) {
	frozen := func() *khipu.FrozenKhipu {
		q, p, k := continuationFixture(t, "aa bb cc dd ee ff", bidi.LeftToRight, false)
		layout := DefaultLayout(linebreak.RectangularParShape(40 * dimen.BP))
		path, err := BreakParagraph(k, layout, nil)
		if err != nil {
			t.Fatal(err)
		}
		c, err := q.Checkpoint(t.Context(), p, path, 1)
		if err != nil {
			t.Fatal(err)
		}
		layout.LineShape = &continuationShape{prefix: []dimen.DU{40 * dimen.BP}, tail: 20 * dimen.BP}
		path, err = ContinueParagraph(t.Context(), k, layout, c)
		if err != nil {
			t.Fatal(err)
		}
		return checkContinuationFrozen(t, q, k, path, layout)
	}() // all working handles are now out of scope
	runtime.GC()
	texts := 0
	for i := 0; i < frozen.KnotCount(); i++ {
		text, ok := frozen.CopyText(i)
		if !ok {
			continue
		}
		texts++
		before := text.Glyphs[0].GlyphID
		text.Glyphs[0].GlyphID++
		again, _ := frozen.CopyText(i)
		if again.Glyphs[0].GlyphID != before {
			t.Fatal("glyphs not immutable")
		}
	}
	if texts != 6 {
		t.Fatal("lost retained text", texts)
	}
}

func TestContinueGeometryFailureDoesNotPublish(t *testing.T) {
	q, p, k := continuationFixture(t, "aa bb cc dd ee ff", bidi.LeftToRight, false)
	layout := DefaultLayout(linebreak.RectangularParShape(40 * dimen.BP))
	path, err := BreakParagraph(k, layout, nil)
	if err != nil {
		t.Fatal(err)
	}
	c, err := q.Checkpoint(t.Context(), p, path, 1)
	if err != nil {
		t.Fatal(err)
	}
	k.SelectedDiscretionaries[0] = khipu.DiscretionarySelection{Source: 0, Variant: 9}
	before := maps.Clone(k.SelectedDiscretionaries)
	bad := layout
	bad.LeftSkip = khipu.KnotCore{W: 0, MinW: 1, MaxW: 1}
	if got, err := ContinueParagraph(t.Context(), k, bad, c); got != nil || err == nil || !maps.Equal(before, k.SelectedDiscretionaries) {
		t.Fatal(got, err)
	}
	if got, err := ContinueParagraph(t.Context(), k, layout, c); err != nil || !slices.Equal(got, path) || len(k.SelectedDiscretionaries) != 0 {
		t.Fatal("retry", got, err)
	}
}

type continueCancelContext struct {
	context.Context
	at, calls int
}

func (c *continueCancelContext) Err() error {
	c.calls++
	if c.at > 0 && c.calls >= c.at {
		return context.DeadlineExceeded
	}
	return nil
}

func TestContinueCancellationFailureAndRetry(t *testing.T) {
	q, p, k := continuationFixture(t, "aa bb cc dd ee ff gg hh ii jj", bidi.LeftToRight, false)
	layout := DefaultLayout(linebreak.RectangularParShape(40 * dimen.BP))
	path, err := BreakParagraph(k, layout, nil)
	if err != nil {
		t.Fatal(err)
	}
	c, err := q.Checkpoint(t.Context(), p, path, 1)
	if err != nil {
		t.Fatal(err)
	}
	layout.LineShape = &continuationShape{prefix: []dimen.DU{40 * dimen.BP}, tail: 20 * dimen.BP}
	probe := &continueCancelContext{Context: t.Context()}
	want, err := ContinueParagraph(probe, k, layout, c)
	if err != nil {
		t.Fatal(err)
	}
	for at := 1; at <= probe.calls; at++ {
		// Sentinel selections expose clearing or publication on any failed call.
		k.SelectedDiscretionaries[0] = khipu.DiscretionarySelection{Source: 0, Variant: 9}
		before := maps.Clone(k.SelectedDiscretionaries)
		ctx := &continueCancelContext{Context: t.Context(), at: at}
		if got, err := ContinueParagraph(ctx, k, layout, c); got != nil || !errors.Is(err, context.DeadlineExceeded) || !maps.Equal(before, k.SelectedDiscretionaries) {
			t.Fatal(at, got, err)
		}
		got, err := ContinueParagraph(t.Context(), k, layout, c)
		if err != nil || !slices.Equal(got, want) || len(k.SelectedDiscretionaries) > 0 {
			t.Fatal("retry", got, err)
		}
	}
	for _, seed := range []*khipu.BreakCheckpoint{nil, {}} {
		if got, err := ContinueParagraph(t.Context(), k, layout, seed); got != nil || !errors.Is(err, khipu.ErrIllegalArguments) {
			t.Fatal(got, err)
		}
	}
	if got, err := ContinueParagraph(nil, k, layout, c); got != nil || !errors.Is(err, khipu.ErrIllegalArguments) {
		t.Fatal(got, err)
	}
	if got, err := ContinueParagraph(t.Context(), k, khipu.Params{}, c); got != nil || !errors.Is(err, khipu.ErrIllegalArguments) {
		t.Fatal(got, err)
	}
}
