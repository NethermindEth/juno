package service

import "context"

type Service interface {
	// Run starts the service. Services should not return non-critical errors,
	// instead they should log and retry.
	//
	// A long-running service should block until ctx is cancelled and then return
	// nil. A short-lived service may return nil once its work is done; a clean
	// return does not shut the node down. Returning a (critical) error or
	// panicking triggers a node-wide shutdown.
	Run(ctx context.Context) error
}
