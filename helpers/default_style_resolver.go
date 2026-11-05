package helpers

import (
	"fmt"
	"reflect"
	"slices"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/contracts"
)

// DefaultStyleResolver validates and copies defaults for unstyled runs. Its
// zero value is ready to use and may be shared concurrently. It rejects all
// non-nil styles because it does not interpret application-defined styles.
// It does not infer properties, acquire a scale, or validate backend features.
// This helper is independent of the encoder, which uses contracts.Shaper directly.
type DefaultStyleResolver struct{}

// NewDefaultStyleResolver returns a stateless resolver for unstyled runs.
func NewDefaultStyleResolver() *DefaultStyleResolver {
	return &DefaultStyleResolver{}
}

// ResolveStyle requires a non-nil font (including its underlying value), a
// positive em size, and a declared spacing mode. It preserves every property
// and clones Features so the result does not share mutable slice storage with
// the caller or other results. Errors wrap contracts.ErrIllegalArguments and
// return no usable style. A later Font.Scale or shaping call may still reject
// the font or its features; resolution does not call into the backend.
func (DefaultStyleResolver) ResolveStyle(style styled.Style, defaults contracts.ResolvedRunStyle) (contracts.ResolvedRunStyle, error) {
	if style != nil {
		return contracts.ResolvedRunStyle{}, fmt.Errorf("%w: default resolver does not interpret non-nil styles", contracts.ErrIllegalArguments)
	}
	if isNilInterface(defaults.Font) || defaults.EmSize <= 0 {
		return contracts.ResolvedRunStyle{}, fmt.Errorf("%w: style requires a font and positive em size", contracts.ErrIllegalArguments)
	}
	if defaults.Spacing != contracts.SpacingFixed && defaults.Spacing != contracts.SpacingElastic {
		return contracts.ResolvedRunStyle{}, fmt.Errorf("%w: unknown spacing mode", contracts.ErrIllegalArguments)
	}
	defaults.Features = slices.Clone(defaults.Features)
	return defaults, nil
}

// Interfaces can carry a typed nil pointer, map, or other nilable backend type.
// Reject those without calling a method that might dereference that value.
func isNilInterface(v any) bool {
	if v == nil {
		return true
	}
	switch rv := reflect.ValueOf(v); rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	}
	return false
}

var _ contracts.StyleResolver = DefaultStyleResolver{}
