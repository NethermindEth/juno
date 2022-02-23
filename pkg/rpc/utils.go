package rpc

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
)

func StructPrinter(i interface{}) {
	b, err := json.Marshal(i)
	if err != nil {
		log.WithField("Error", err).Error("Error marshaling interface")
		return
	}
	log.WithField("Struct", string(b)).Info("Struct as a dictionary")
}
