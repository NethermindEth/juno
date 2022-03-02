package log

import (
	"go.uber.org/zap"
	"sync"
)

var once sync.Once

var sugaredLogger *zap.SugaredLogger

// GetLogger returns logger for the app using Singleton Pattern Design
func GetLogger() *zap.SugaredLogger {
	once.Do(func() {
		// See https://pkg.go.dev/go.uber.org/zap#hdr-Choosing_a_Logger for more configurations
		logger, err := zap.NewDevelopment()
		if err != nil {
			panic(err)
		}
		sugaredLogger = logger.Sugar()
	})
	return sugaredLogger
}
