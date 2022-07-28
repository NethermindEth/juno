package gateway

import (
	. "github.com/NethermindEth/juno/internal/log"
	"go.uber.org/zap"
)

// NewLogger returns the global logger in internal/log.
func NewLogger() *zap.SugaredLogger {
	return Logger
}

// NewNoOpLogger returns a *zap.SugaredLogger that discards any output
// it is asked to log.
func NewNoOpLogger() *zap.SugaredLogger {
	return zap.NewNop().Sugar()
}
