// Package log provides a logger.
package log

import "go.uber.org/zap"

// Default is the default logger. It is a "sugared" variant of the zap
// logger.
var Default *zap.SugaredLogger

func init() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	Default = logger.Sugar()
}
