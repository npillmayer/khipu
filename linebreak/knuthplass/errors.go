package knuthplass

import "errors"

var (
	ErrNoParShape    = errors.New("Cannot shape a paragraph without a LineShape")
	ErrNoBreakpoints = errors.New("No breakpoints could be found for paragraph")
)
