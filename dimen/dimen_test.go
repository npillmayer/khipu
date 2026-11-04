package dimen

import "testing"

func TestBigPointString(t *testing.T) {
	if got := BP.String(); got != "65536sp" {
		t.Errorf("a big point should be 65536 scaled points, got %q", got)
	}
}
