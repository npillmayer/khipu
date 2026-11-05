// Package contracts defines the font, shaping, and style services consumed by
// khipu, including their parameter and result types. It owns no paragraph data
// and does not import the root khipu package or concrete helper implementations.
//
// Shaper is the active metric boundary. StreamingShaper and StyleResolver
// describe draft boundaries with standalone implementations in package helpers;
// their integration into paragraph encoding remains future work. Paragraph-
// specific interfaces such as KnotStyler and Hyphenator stay in package khipu.
package contracts

import "errors"

// ErrIllegalArguments identifies invalid arguments across the engine and its
// service implementations. Implementations may wrap it with further context.
var ErrIllegalArguments = errors.New("khipu: illegal arguments")
