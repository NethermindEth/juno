// Package fast holds the primitives of Juno's custom CBOR decoder: a cursor over the input
// and bounds-checked readers for heads, integers, strings, tags and big integers, plus a
// Skip that checks the syntax of what it passes over.
//
// Decoding is single pass. Every length, count, nesting depth and UTF-8 check happens
// inline, so malformed input returns an error and never panics, hangs or allocates more
// than the input can back. The values the readers return are copies; only Key and Remaining return a slice of the input.
//
// The package sits below core: it must not import core, core/felt or the cbor wrapper.
package fast
