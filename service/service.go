package service

import "context"

type Service interface {
	// Run starts the service. Services should not return non-critical errors,
	// instead they should log and retry. If a service returns an error the node
	// shuts down. If a service returns a nil error, it is assumed its work has
	// finished safely.
	Run(ctx context.Context) error
}
