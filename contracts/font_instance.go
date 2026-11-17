package contracts

import (
	"fmt"
	"math"
	"reflect"

	"github.com/npillmayer/khipu/dimen"
)

// FontInstance binds an immutable face, physical em size and its conversion.
// Its zero value is invalid. Copies share face/scale services, which must remain
// immutable and usable throughout all retained results' lifetimes.
// This binds size only, not variations, synthetic styling or rasterization.
type FontInstance struct {
	face  FontFace
	size  dimen.DU
	scale FontScale
}

// NewFontInstance binds size once. It performs no loading or metric queries.
// Scale is called exactly once; backend errors retain their identity.
func NewFontInstance(face FontFace, size dimen.DU) (FontInstance, error) {
	if nilFontValue(face) || size <= 0 || size >= dimen.Fil {
		return FontInstance{}, fmt.Errorf("%w: invalid font face or em size", ErrIllegalArguments)
	}
	scale, err := face.Scale(size)
	if err != nil {
		return FontInstance{}, fmt.Errorf("font instance: %w", err)
	}
	if nilFontValue(scale) {
		return FontInstance{}, fmt.Errorf("%w: nil font scale", ErrIllegalArguments)
	}
	unit := scale.ToDU(1)
	if unit <= 0 || math.IsNaN(unit) || math.IsInf(unit, 0) {
		return FontInstance{}, fmt.Errorf("%w: invalid font scale", ErrIllegalArguments)
	}
	return FontInstance{face: face, size: size, scale: scale}, nil
}

// Valid reports construction validity without calling any backend service.
func (f FontInstance) Valid() bool { return f.size > 0 }

// Face returns the shared size-independent face (nil for the zero value).
func (f FontInstance) Face() FontFace { return f.face }

// EmSize returns the physical em size in DU (zero for the zero value).
func (f FontInstance) EmSize() dimen.DU { return f.size }

// Scale returns the retained conversion (nil for the zero value).
func (f FontInstance) Scale() FontScale { return f.scale }

func nilFontValue(v any) bool {
	if v == nil {
		return true
	}
	r := reflect.ValueOf(v)
	switch r.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return r.IsNil()
	}
	return false
}
