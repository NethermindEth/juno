package l1

// Subscription mirrors go-ethereum's event.Subscription surface as
// consumed by l1.Client: a channel signalling failure, and an
// Unsubscribe method to release resources.
type Subscription interface {
	Err() <-chan error
	Unsubscribe()
}
