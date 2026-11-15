// Package contracts defines the font, shaping, and style services consumed by
// khipu, including their parameter and result types. It owns no paragraph data
// and does not import the root khipu package or concrete helper implementations.
//
// StreamingShaper and StyleResolver drive paragraph encoding and lazy
// discretionary reshaping. Package helpers supplies synthetic implementations.
// Paragraph-specific interfaces such as Hyphenator stay in package khipu.
// AtomicID and AtomicAnchor separately identify opaque inline occurrences in
// source text. They carry no renderer payload, dimensions, or shaping service.
package contracts

import "errors"

// ErrIllegalArguments identifies invalid arguments across the engine and its
// service implementations. Implementations may wrap it with further context.
var ErrIllegalArguments = errors.New("khipu: illegal arguments")
