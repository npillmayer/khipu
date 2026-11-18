package khipu

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func checkpointFixture(t *testing.T, q *Khipukamayuq, text string) (*PreparedParagraph, *Khipu, []int) {
	t.Helper()
	p, err := q.PrepareParagraph(t.Context(), ParagraphInput{Paragraph: preparationParagraph(t, text)}, PrepareOptions{})
	if err != nil {
		t.Fatal(err)
	}
	k, err := q.BindParagraph(t.Context(), p, nil)
	if err != nil {
		t.Fatal(err)
	}
	// A complete legal path with one line at every permitted boundary.
	var path []int
	for i, c := range p.knots {
		if c.Penalty < MaxPenalty {
			path = append(path, i)
		}
	}
	return p, k, path
}

func TestCheckpointCertification(t *testing.T) {
	q := NewKhipukamayuq(Config{})
	p, k, path := checkpointFixture(t, q, "aa bb\ncc dd ee")
	for _, count := range []int{0, 1, len(path)} {
		c, err := q.Checkpoint(t.Context(), p, path, count)
		if err != nil {
			t.Fatal(err)
		}
		prefix, err := c.PrefixFor(t.Context(), k)
		if err != nil || !slices.Equal(prefix, path[:count]) {
			t.Fatal(prefix, err)
		}
		if count > 0 {
			prefix[0] = -1
		}
		again, err := c.PrefixFor(t.Context(), k)
		if err != nil || !slices.Equal(again, path[:count]) {
			t.Fatal("getter aliases checkpoint", again, err)
		}
	}
	c, err := q.Checkpoint(t.Context(), p, path, 1)
	if err != nil {
		t.Fatal(err)
	}
	want := path[0]
	path[0] = -1
	k.Penalty = slices.Clone(k.Penalty)
	k.Penalty[want] = MaxPenalty // deliberately corrupt only the old binding
	fresh, err := q.BindParagraph(t.Context(), p, nil)
	if err != nil {
		t.Fatal(err)
	}
	prefix, err := c.PrefixFor(t.Context(), fresh)
	if err != nil || !slices.Equal(prefix, []int{want}) {
		t.Fatal("checkpoint retained working arrays", prefix, err)
	}
}

func TestCheckpointRejectsInvalidPathsAndOwners(t *testing.T) {
	q := NewKhipukamayuq(Config{})
	p, k, path := checkpointFixture(t, q, "aa bb\ncc dd ee")
	var prohibited, forced int
	for i, c := range p.knots {
		if c.Penalty >= MaxPenalty {
			prohibited = i
		}
		if c.Penalty <= MaxMerit && i < len(p.knots)-1 {
			forced = i
		}
	}
	withoutForced := slices.DeleteFunc(slices.Clone(path), func(i int) bool { return i == forced })
	for _, bad := range [][]int{
		nil, path[:len(path)-1], {-1, len(p.knots) - 1}, {len(p.knots)},
		{path[0], path[0], len(p.knots) - 1}, {path[1], path[0], len(p.knots) - 1},
		{prohibited, len(p.knots) - 1}, withoutForced,
	} {
		// Even an empty accepted prefix must validate the entire path.
		if c, err := q.Checkpoint(t.Context(), p, bad, 0); c != nil || !errors.Is(err, ErrIllegalArguments) {
			t.Fatal(bad, c, err)
		}
	}
	for _, count := range []int{-1, len(path) + 1} {
		if c, err := q.Checkpoint(t.Context(), p, path, count); c != nil || !errors.Is(err, ErrIllegalArguments) {
			t.Fatal(c, err)
		}
	}
	for _, owner := range []*Khipukamayuq{nil, NewKhipukamayuq(Config{})} {
		if c, err := owner.Checkpoint(t.Context(), p, path, 1); c != nil || !errors.Is(err, ErrIllegalArguments) {
			t.Fatal(c, err)
		}
	}
	for _, bad := range []*PreparedParagraph{nil, {}, {owner: q}} {
		if c, err := q.Checkpoint(t.Context(), bad, path, 1); c != nil || !errors.Is(err, ErrIllegalArguments) {
			t.Fatal(c, err)
		}
	}
	c, err := q.Checkpoint(t.Context(), p, path, 1)
	if err != nil {
		t.Fatal(err)
	}
	_, sameOwnerOther, _ := checkpointFixture(t, q, "aa bb\ncc dd ee")
	_, foreign, _ := checkpointFixture(t, NewKhipukamayuq(Config{}), "aa bb\ncc dd ee")
	legacy, err := q.EncodeParagraph(t.Context(), preparationParagraph(t, "aa bb\ncc dd ee"))
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []*Khipu{nil, {}, sameOwnerOther, foreign, legacy} {
		if got, err := c.PrefixFor(t.Context(), bad); got != nil || !errors.Is(err, ErrIllegalArguments) {
			t.Fatal(got, err)
		}
	}
	for _, bad := range []*BreakCheckpoint{nil, {}} {
		if got, err := bad.PrefixFor(t.Context(), k); got != nil || !errors.Is(err, ErrIllegalArguments) {
			t.Fatal(got, err)
		}
	}
	k.W = k.W[:len(k.W)-1]
	if got, err := c.PrefixFor(t.Context(), k); got != nil || !errors.Is(err, ErrIllegalArguments) {
		t.Fatal(got, err)
	}
}

type checkpointCancel struct {
	context.Context
	calls, at int
}

func (c *checkpointCancel) Err() error {
	c.calls++
	if c.at > 0 && c.calls >= c.at {
		return context.DeadlineExceeded
	}
	return nil
}

func TestCheckpointPolicyCancellationAndRetry(t *testing.T) {
	q := NewKhipukamayuq(Config{})
	p, k, path := checkpointFixture(t, q, "aa bb cc dd ee")
	for _, mode := range []HyphenationMode{ExplicitHyphenation, AutomaticHyphenation} {
		copy := *p
		copy.hyphenation = mode
		if c, err := q.Checkpoint(t.Context(), &copy, path, 1); c != nil || !errors.Is(err, ErrUnsupportedPolicy) {
			t.Fatal(c, err)
		}
	}
	probe := &checkpointCancel{Context: t.Context()}
	c, err := q.Checkpoint(probe, p, path, 1)
	if err != nil {
		t.Fatal(err)
	}
	for at := 1; at <= probe.calls; at++ {
		ctx := &checkpointCancel{Context: t.Context(), at: at}
		if got, err := q.Checkpoint(ctx, p, path, 1); got != nil || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal(at, got, err)
		}
	}
	if got, err := q.Checkpoint(nil, p, path, 1); got != nil || !errors.Is(err, ErrIllegalArguments) {
		t.Fatal(got, err)
	}
	for _, ctx := range []context.Context{nil, &checkpointCancel{Context: t.Context(), at: 1}, &checkpointCancel{Context: t.Context(), at: 2}} {
		if got, err := c.PrefixFor(ctx, k); got != nil || err == nil {
			t.Fatal(got, err)
		}
	}
	if got, err := c.PrefixFor(t.Context(), k); err != nil || !slices.Equal(got, path[:1]) {
		t.Fatal("retry", got, err)
	}
	k.hyphenation = ExplicitHyphenation
	if got, err := c.PrefixFor(t.Context(), k); got != nil || !errors.Is(err, ErrUnsupportedPolicy) {
		t.Fatal(got, err)
	}
}
