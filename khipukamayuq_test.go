package khipu

import (
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/schuko/tracing/gotestingadapter"
	"github.com/npillmayer/uax/bidi"
)

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
