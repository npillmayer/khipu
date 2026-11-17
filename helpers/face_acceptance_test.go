package helpers_test

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
)

type acceptanceTrap []byte

func (acceptanceTrap) Metrics() (contracts.FontMetrics, error) { panic("acceptance queried metrics") }
func (acceptanceTrap) Scale(dimen.DU) (contracts.FontScale, error) {
	panic("acceptance queried scale")
}

func TestSyntheticFaceAcceptanceIsPure(t *testing.T) {
	s := testStream(t, 8)
	if err := s.SetInitialCluster(9); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		face contracts.FontFace
		want error
	}{
		{helpers.SyntheticFont{}, nil}, {helpers.NewSyntheticFont(), nil},
		{nil, contracts.ErrIllegalArguments}, {(*helpers.SyntheticFont)(nil), contracts.ErrIllegalArguments},
		{acceptanceTrap(nil), contracts.ErrIllegalArguments}, {acceptanceTrap{1}, contracts.ErrUnsupportedFontFace},
	} {
		if err := s.CheckFace(tc.face); !errors.Is(err, tc.want) {
			t.Fatal(err, tc.want)
		}
	}
	if s.Err() != nil {
		t.Fatal("check poisoned stream")
	}
	select {
	case <-s.Glyphs():
		t.Fatal("check emitted or closed output")
	default:
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 100 {
				if err := s.CheckFace(helpers.SyntheticFont{}); err != nil {
					t.Error(err)
				}
			}
		})
	}
	if err := s.Shape(t.Context(), syntheticParams(), strings.NewReader("a"), contracts.ShapeBufferOptions{}); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	if g := <-s.Glyphs(); g.Cluster != 9 {
		t.Fatal("changed cluster base", g)
	}
	if err := s.Finish(); err != nil {
		t.Fatal(err)
	}
	if err := s.CheckFace(helpers.NewSyntheticFont()); err != nil {
		t.Fatal(err)
	}
	if err := s.Finish(); err != nil {
		t.Fatal(err)
	}
	// Direct Shape retains the historical argument classification and terminal error.
	direct := testStream(t, 1)
	p := syntheticParams()
	p.Font = testInstance(foreignSizedFace{}, 10*dimen.BP)
	err := direct.Shape(t.Context(), p, strings.NewReader("a"), contracts.ShapeBufferOptions{})
	if !errors.Is(err, contracts.ErrIllegalArguments) || !errors.Is(err, contracts.ErrUnsupportedFontFace) || !errors.Is(direct.Err(), contracts.ErrUnsupportedFontFace) {
		t.Fatal(err, direct.Err())
	}
}
