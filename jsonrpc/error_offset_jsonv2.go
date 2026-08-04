//go:build goexperiment.jsonv2

// TODO(granza): Move the content of this file into pretty_error.go once jsonv2 is the default.
//
// jsonv2 always reports an error at the first byte of whatever caused it.

package jsonrpc

const errorOffsetAdjustment = 0
