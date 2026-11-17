package contracts_test

import (
	"errors"
	"math"
	"sync"
	"testing"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
)

type testScale float64

func (s testScale) ToDU(n int32) float64 { return float64(s) * float64(n) }

type nilScale struct{}

func (*nilScale) ToDU(int32) float64 { panic("nil scale called") }

type instanceFace struct {
	calls int
	upem  float64
	scale contracts.FontScale
	err   error
}

func (*instanceFace) Metrics() (contracts.FontMetrics, error) { panic("construction queried metrics") }
func (f *instanceFace) Scale(size dimen.DU) (contracts.FontScale, error) {
	f.calls++
	if f.err != nil || f.scale != nil {
		return f.scale, f.err
	}
	upem := f.upem
	if upem == 0 {
		upem = 1000
	}
	return testScale(float64(size) / upem), nil
}
func TestInstanceBindingAndCopies(t *testing.T) {
	f := new(instanceFace)
	a, err := contracts.NewFontInstance(f, 12*dimen.BP)
	if err != nil {
		t.Fatal(err)
	}
	b, err := contracts.NewFontInstance(f, 24*dimen.BP)
	if err != nil {
		t.Fatal(err)
	}
	c := a
	for _, i := range []contracts.FontInstance{a, b, c} {
		if !i.Valid() || i.Face() != f || i.Scale().ToDU(-250) != -float64(i.EmSize())/4 {
			t.Fatal("incorrect binding", i)
		}
	}
	if a.Scale().ToDU(1) != float64(12*dimen.BP)/1000 || a.EmSize() != c.EmSize() || f.calls != 2 {
		t.Fatal("size changed existing instance")
	}
	// Concurrent accessor reads must not invoke the backend.
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 100 {
				_ = a.Valid()
				_ = a.Face()
				_ = a.EmSize()
				_ = a.Scale()
			}
		})
	}
	wg.Wait()
	if f.calls != 2 {
		t.Fatal("accessor queried scale")
	}
}

type nilReturningFace struct{ instanceFace }

func (*nilReturningFace) Scale(dimen.DU) (contracts.FontScale, error) { return nil, nil }
func TestInstanceValidation(t *testing.T) {
	sentinel := errors.New("scale failure")
	var typedNil *instanceFace
	for _, tc := range []struct {
		face contracts.FontFace
		size dimen.DU
		want error
	}{
		{nil, 1, contracts.ErrIllegalArguments}, {typedNil, 1, contracts.ErrIllegalArguments},
		{new(instanceFace), 0, contracts.ErrIllegalArguments}, {new(instanceFace), -1, contracts.ErrIllegalArguments},
		{new(instanceFace), dimen.Fil, contracts.ErrIllegalArguments},
		{&instanceFace{err: sentinel}, 1, sentinel},
		{new(nilReturningFace), 1, contracts.ErrIllegalArguments},
		{&instanceFace{scale: (*nilScale)(nil)}, 1, contracts.ErrIllegalArguments},
		{&instanceFace{scale: testScale(0)}, 1, contracts.ErrIllegalArguments},
		{&instanceFace{scale: testScale(-1)}, 1, contracts.ErrIllegalArguments},
		{&instanceFace{scale: testScale(math.NaN())}, 1, contracts.ErrIllegalArguments},
		{&instanceFace{scale: testScale(math.Inf(1))}, 1, contracts.ErrIllegalArguments},
	} {
		i, err := contracts.NewFontInstance(tc.face, tc.size)
		if i.Valid() || !errors.Is(err, tc.want) {
			t.Fatalf("accepted invalid binding: %+v, %v", tc, err)
		}
	}
	var zero contracts.FontInstance
	if zero.Valid() || zero.Face() != nil || zero.Scale() != nil || zero.EmSize() != 0 {
		t.Fatal("zero value")
	}
	f := new(instanceFace)
	if _, err := contracts.NewFontInstance(f, 0); err == nil || f.calls != 0 {
		t.Fatal("invalid size called backend")
	}
}

func TestInstanceUsesSelectedFacesUnits(t *testing.T) {
	primary := &instanceFace{upem: 1000}
	fallback := &instanceFace{upem: 2048}
	a, err := contracts.NewFontInstance(primary, 12*dimen.BP)
	if err != nil {
		t.Fatal(err)
	}
	b, err := contracts.NewFontInstance(fallback, 12*dimen.BP)
	if err != nil {
		t.Fatal(err)
	}
	if a.Scale().ToDU(500) != b.Scale().ToDU(1024) || a.Scale().ToDU(500) != float64(6*dimen.BP) {
		t.Fatal("conversion not bound to selected face")
	}
}
