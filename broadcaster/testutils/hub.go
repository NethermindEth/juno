package testutils

import (
	"github.com/NethermindEth/juno/broadcaster"
)

// NewHub builds a hub on the backend selected by [Kind], so a test written once
// runs on whichever backend JUNO_BROADCAST selects.
func NewHub[T any](opts ...broadcaster.Option) broadcaster.BroadcastHub[T] {
	return broadcaster.New[T](append([]broadcaster.Option{broadcaster.WithKind(Kind())}, opts...)...)
}
