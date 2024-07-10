package log

import (
	"NEWzDNS/config"
	"fmt"
	"github.com/miekg/dns"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"net"
	"os"
	"path/filepath"
	"strings"
)

var (
	RequestLogger *zap.SugaredLogger
	ErrorLogger   *zap.SugaredLogger
	BlockLogger   *zap.SugaredLogger
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
	blockLogger := newLogger(filepath.Join(logDir, "blocked.log"))
	if blockLogger == nil {
		fmt.Println("Failed to initialize error logger")
		return fmt.Errorf("failed to initialize error logger")
	}
	BlockLogger = blockLogger

	return nil
}

func newLogger(logPath string) *zap.SugaredLogger {
	lj := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    config.Cfg.Server.LogMaxSize,
		MaxBackups: config.Cfg.Server.LogMaxBackups,
		Compress:   true,
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
func RequestInfo(w dns.ResponseWriter, domain string, response *dns.Msg, upstream string) {
	if !config.Cfg.Server.EnableLogging {
		return
	}

	clientIP, _, err := net.SplitHostPort(w.RemoteAddr().String())
	if err != nil {
		clientIP = "unknown"
	}
	var resolvedResults strings.Builder

	for _, answer := range response.Answer {
		switch a := answer.(type) {
		case *dns.A:
			if resolvedResults.Len() > 0 {
				resolvedResults.WriteString(", ")
			}
			resolvedResults.WriteString(a.A.String())
		case *dns.AAAA:
			if resolvedResults.Len() > 0 {
				resolvedResults.WriteString(", ")
			}
			resolvedResults.WriteString(a.AAAA.String())
		}
	}

	if resolvedResults.Len() > 0 {
		RequestLogger.Info(
			"client_ip:", clientIP,
			" domain:", domain,
			" resolved_results:", resolvedResults.String(),
			" upstream:", upstream,
		)
	}
}
func Sync() {
	if RequestLogger != nil {
		err := RequestLogger.Sync()
		if err != nil {
			return
		}
	}
	if ErrorLogger != nil {
		err := ErrorLogger.Sync()
		if err != nil {
			return
		}
	}
	if BlockLogger != nil {
		err := BlockLogger.Sync()
		if err != nil {
			return
		}
	}
}
