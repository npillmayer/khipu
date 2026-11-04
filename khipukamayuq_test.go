package khipu

import (
	"io"
	"strings"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/schuko/tracing/gotestingadapter"
	"github.com/npillmayer/uax/bidi"
)

func TestNewKhipukamayuqDefaultShaper(t *testing.T) {
	first := NewKhipukamayuq(Config{})
	second := NewKhipukamayuq(Config{})
	first.env.shaper.SetSource(strings.NewReader("x"))
	cluster, width, size, err := first.env.shaper.ReadClusterMetrics()
	if err != nil || cluster != 0 || width != 5*dimen.PT || size != 1 {
		t.Fatalf("unexpected default shaper metrics: %d, %v, %d, %v", cluster, width, size, err)
	}
	// The default shaper is stateful and must be private to each owner.
	if _, _, _, err := second.env.shaper.ReadClusterMetrics(); err != io.EOF {
		t.Fatalf("second owner's shaper should have no source, got %v", err)
	}
}

func TestParaSimple(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "tyse.khipu")
	defer teardown()
	//
	text := styled.TextFromString("The quick brown fox jumps over the lazy dog")
	bold := teststyle("bold")
	var err error
	if text, err = text.Style(bold, 4, 15); err != nil {
		t.Fatalf("initial styling failed: %v", err)
	}
	para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), bidi.LeftToRight, nil)
	kq := newTestKq()
	khipu, err := kq.EncodeParagraph(para, nil)
	if err != nil {
		t.Fatalf("encoding paragraph failed: %v", err)
	}
	if khipu == nil {
		t.Fatalf("khipu is nil")
	}
	//t.Logf("khipu: %v", khipu)
	//t.Fail()
}
