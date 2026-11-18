package service

import "context"

type Service interface {
	// Run starts the service. Services should not return non-critical errors,
	// instead they should log and retry. If Run returns an error, the node
	// stops. If Run returns nil, only this service stops.
	Run(ctx context.Context) error
}
