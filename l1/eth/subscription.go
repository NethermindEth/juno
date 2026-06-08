package eth

// Subscription mirrors go-ethereum's event.Subscription surface as
// consumed by juno's l1 package: a channel signalling failure, and an
// Unsubscribe method to release resources.
type Subscription interface {
	Err() <-chan error
	Unsubscribe()
}
