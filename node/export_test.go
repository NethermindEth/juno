package node

import "github.com/NethermindEth/juno/utils/log"

// NewWithLogger builds a minimal Node for tests that only need to exercise
// service lifecycle behaviour and observe what it logs. It is a test-only seam:
// because this file has the _test.go suffix it never becomes part of Juno's
// public API.
func NewWithLogger(logger log.Logger) *Node {
	return &Node{logger: logger}
}
