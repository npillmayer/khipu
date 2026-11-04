package khipu

import (
	"io"
	"strings"
	"testing"

	"github.com/npillmayer/khipu/dimen"
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
