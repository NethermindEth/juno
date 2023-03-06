package service

import "context"

type Service interface {
	// Run starts the service and returns an error. Services should not return non-critical errors,
	// instead they should log and retry. Services should be blocking.
	Run(ctx context.Context) error
}
