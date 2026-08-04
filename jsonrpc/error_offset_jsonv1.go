//go:build !goexperiment.jsonv2

// TODO(granza): Delete this file once jsonv2 is the default.
//
// jsonv1 always reports an error one byte past the end of whatever caused it.

package jsonrpc

const errorOffsetAdjustment = 1
