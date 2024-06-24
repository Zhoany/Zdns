package log

import (
	"NEWzDNS/config"
	"go.uber.org/zap"
)

var (
	RequestLogger *zap.SugaredLogger
	ErrorLogger   *zap.SugaredLogger
)

func InitLogger() error {
	if !config.Cfg.Server.EnableLogging {
		return nil
	}

	requestConfig := zap.NewProductionConfig()
	requestConfig.OutputPaths = []string{"request.log"}
	requestLogger, err := requestConfig.Build()
	if err != nil {
		return err
	}
	RequestLogger = requestLogger.Sugar()

	errorConfig := zap.NewProductionConfig()
	errorConfig.OutputPaths = []string{"error.log"}
	errorLogger, err := errorConfig.Build()
	if err != nil {
		return err
	}
	ErrorLogger = errorLogger.Sugar()

	return nil
}

func Sync() {
	if RequestLogger != nil {
		RequestLogger.Sync()
	}
	if ErrorLogger != nil {
		ErrorLogger.Sync()
	}
}
