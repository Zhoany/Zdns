package log

import (
	"NEWzDNS/config"
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
	"path/filepath"
)

var (
	RequestLogger *zap.SugaredLogger
	ErrorLogger   *zap.SugaredLogger
)

func InitLogger() error {
	if !config.Cfg.Server.EnableLogging {
		fmt.Println("Logging is disabled in the configuration.")
		return nil
	}

	logDir := "logs"
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		err := os.MkdirAll(logDir, 0755)
		if err != nil {
			fmt.Printf("Failed to create log directory: %v\n", err)
			return err
		}
		fmt.Println("Log directory created.")
	}

	requestLogger := newLogger(filepath.Join(logDir, "request.log"))
	if requestLogger == nil {
		fmt.Println("Failed to initialize request logger")
		return fmt.Errorf("failed to initialize request logger")
	}
	RequestLogger = requestLogger

	errorLogger := newLogger(filepath.Join(logDir, "error.log"))
	if errorLogger == nil {
		fmt.Println("Failed to initialize error logger")
		return fmt.Errorf("failed to initialize error logger")
	}
	ErrorLogger = errorLogger

	return nil
}

func newLogger(logPath string) *zap.SugaredLogger {
	lj := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    config.Cfg.Server.LogMaxSize,    // 每个日志文件最大100 MB
		MaxBackups: config.Cfg.Server.LogMaxBackups, // 保留最多7个备份
		MaxAge:     7,                               // 保留7天
		Compress:   true,                            // 启用压缩
	}

	w := zapcore.AddSync(lj)
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		w,
		zap.InfoLevel,
	)

	logger := zap.New(core)
	return logger.Sugar()
}

func Sync() {
	if RequestLogger != nil {
		RequestLogger.Sync()
	}
	if ErrorLogger != nil {
		ErrorLogger.Sync()
	}
}
