package client

import "errors"

// ErrTransportClosed indicates the underlying transport has been shut
// down (cleanly via Close or via an upstream connection failure).
// Surfaced to in-flight calls and active subscriptions.
var ErrTransportClosed = errors.New("transport closed")
