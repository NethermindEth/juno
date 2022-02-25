package rpc

import (
	"encoding/json"
	"github.com/NethermindEth/juno/internal/log"
)

var logger = log.GetLogger()

func StructPrinter(i interface{}) {
	b, err := json.Marshal(i)
	if err != nil {
		logger.With("Error", err).Error("Error marshaling interface")
		return
	}
	logger.With("Struct", string(b)).Info("Struct as a dictionary")
}
