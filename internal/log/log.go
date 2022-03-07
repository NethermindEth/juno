package log

import "go.uber.org/zap"

var Default *zap.SugaredLogger

func init() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	Default = logger.Sugar()
}
