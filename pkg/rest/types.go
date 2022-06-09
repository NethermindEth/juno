package rest

import (
	"github.com/NethermindEth/juno/pkg/feeder"
)

type RestHandler struct {
	RestFeeder *feeder.Client
}
