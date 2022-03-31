// Package log provides a logger.
package log

import (
	"log"

	"go.uber.org/zap"
)

// Default is the default logger. It is a "sugared" variant of the zap
// logger.
var Default *zap.SugaredLogger

func init() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		// notest
		log.Fatalln("failed to initialise application logger")
	}
	Default = logger.Sugar()
}
