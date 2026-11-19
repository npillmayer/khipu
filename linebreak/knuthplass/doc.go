// Package knuthplass breaks immutable paragraph bindings into independent Tikrays.
// BreakParagraph accepts khipu.BreakRequest and typed Parameters; nil parameters
// select resolved algorithm defaults. Geometry is explicit and captured once per
// queried line. Discovery supplies immutable replacement opportunities. Search
// state and selections stay private to each invocation; errors return nil plans.
// Freeze the result with khipu.Freeze, without another geometry callback.
// The first successful KP pass wins. Emergency stretch affects scoring only;
// physical allocation uses actual capacities. Seeded KP is not supported.
// Public typesetting calls are synchronous and have no context argument.
package knuthplass
