package gateway

import "go.uber.org/zap"

func NewLogger() *zap.SugaredLogger {
	// TODO: Use consistent output format.
	logger, err := zap.NewProduction()
	if err != nil {
		panic("gateway: failed to initialise logger")
	}
	// TODO: This should probably be defined in the method that shuts down
	// this server.
	// defer logger.Sync()

	return logger.Sugar()
}

func NewNoOpLogger() *zap.SugaredLogger {
	return zap.NewNop().Sugar()
}
