package knuthplass

import "errors"

var (
	// ErrNoParShape reports a nil layout LineShape during breaker setup.
	// Other invalid geometry, including a typed-nil shape, may instead wrap
	// khipu.ErrIllegalArguments. Use errors.Is when checking returned errors.
	ErrNoParShape = errors.New("Cannot shape a paragraph without a LineShape")
	// ErrNoBreakpoints reports empty input or failure to find a complete path
	// through the configured passes. Provider errors are returned separately.
	ErrNoBreakpoints = errors.New("No breakpoints could be found for paragraph")
)
