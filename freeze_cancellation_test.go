package khipu

import (
	"context"
	"errors"
	"testing"

	"github.com/npillmayer/khipu/dimen"
)

func TestFreezeStageCancellation(t *testing.T) {
	owner, k := logicalTestParagraph(t, "aa\nbb")
	breaks := []int{1, len(k.Kind) - 1}
	l, err := owner.materializeLogicalLines(t.Context(), k, breaks)
	if err != nil {
		t.Fatal(err)
	}
	v, err := l.visualLines(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	params := Params{LineShape: geometryShapeFunc(func(int32) dimen.DU { return 30 * dimen.PT })}
	g, err := v.prepareGeometry(t.Context(), params)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if out, err := owner.materializeLogicalLines(ctx, k, breaks); out != nil || !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if out, err := l.lineOrderings(ctx); out != nil || !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if out, err := l.visualLines(ctx); out != nil || !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if out, err := v.prepareGeometry(ctx, params); out != nil || !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if out, err := g.freeze(ctx); out != nil || !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := g.freeze(t.Context()); err != nil {
		t.Fatal("stage snapshot not retryable", err)
	}
}
