package logger

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"ZZDNS/config"

	"golang.org/x/exp/slog"
)

type LogMessage struct {
	Level   slog.Level
	Message string
	Time    time.Time
}

// Logger represents a custom logger
type Logger struct {
	messages   chan LogMessage
	wg         sync.WaitGroup
	logFile    string
	maxSize    int64
	maxBackups int
	file       *os.File
	batchSize  int
}

var (
	instance *Logger
	once     sync.Once
)

// NewLogger creates a new Logger instance and starts the logging goroutine
func NewLogger(logFile string, maxSize int64, maxBackups int, bufferSize int, batchSize int) (*Logger, error) {
	// 打开或创建日志文件
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	// 创建一个文本处理器
	handler := slog.NewTextHandler(f, nil)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	l := &Logger{
		messages:   make(chan LogMessage, bufferSize),
		logFile:    logFile,
		maxSize:    maxSize,
		maxBackups: maxBackups,
		file:       f,
		batchSize:  batchSize,
	}

	// 启动日志处理goroutine
	l.wg.Add(1)
	go l.processMessages()

	return l, nil
}

// initLogger initializes the global logger instance
func initLogger() {
	var err error
	cfg := config.CFG.Logging
	if cfg.File == "" {
		cfg.File = "logs/application.log"
	}
	if cfg.MaxSize == 0 {
		cfg.MaxSize = 500 * 1024 * 1024
	}
	if cfg.MaxBackups == 0 {
		cfg.MaxBackups = 7
	}
	if cfg.BufferSize == 0 {
		cfg.BufferSize = 100
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = 10
	}
	instance, err = NewLogger(cfg.File, cfg.MaxSize, cfg.MaxBackups, cfg.BufferSize, cfg.BatchSize)
	if err != nil {
		log.Fatalf("Error initializing logger: %v", err)
	}
}

// GetLogger returns the singleton instance of Logger
func GetLogger() *Logger {
	once.Do(initLogger)
	return instance
}

// processMessages processes log messages from the channel
func (l *Logger) processMessages() {
	defer l.wg.Done()
	ctx := context.Background()
	buffer := make([]LogMessage, 0, l.batchSize)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	flush := func() {
		for _, msg := range buffer {
			l.rotateLogIfNeeded()
			slog.Log(ctx, msg.Level, msg.Message, "time", msg.Time)
		}
		buffer = buffer[:0]
	}

	for {
		select {
		case msg, ok := <-l.messages:
			if !ok {
				if len(buffer) > 0 {
					flush()
				}
				return
			}
			buffer = append(buffer, msg)
			if len(buffer) >= l.batchSize {
				flush()
			}
		case <-ticker.C:
			if len(buffer) > 0 {
				flush()
			}
		}
	}
}

// rotateLogIfNeeded checks the size of the current log file and rotates if necessary
func (l *Logger) rotateLogIfNeeded() {
	stat, err := l.file.Stat()
	if err != nil {
		log.Printf("Failed to get log file info: %v", err)
		return
	}

	// 如果文件大小超过最大值，则进行轮换
	if stat.Size() >= l.maxSize {
		l.rotateLogs()
	}
}

// rotateLogs rotates the log files
func (l *Logger) rotateLogs() {
	// 关闭当前日志文件
	l.file.Close()

	// 备份旧日志文件
	for i := l.maxBackups - 1; i > 0; i-- {
		oldName := fmt.Sprintf("%s.%d", l.logFile, i)
		newName := fmt.Sprintf("%s.%d", l.logFile, i+1)
		if _, err := os.Stat(oldName); err == nil {
			os.Rename(oldName, newName)
		}
	}

	// 备份当前日志文件为第一个备份
	os.Rename(l.logFile, fmt.Sprintf("%s.1", l.logFile))

	// 创建新的日志文件
	f, err := os.OpenFile(l.logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to create new log file: %v", err)
	}

	// 更新文件和处理器
	l.file = f
	handler := slog.NewTextHandler(f, nil)
	slog.SetDefault(slog.New(handler))
}

// Info logs an info message
func (l *Logger) Info(message string) {
	l.messages <- LogMessage{
		Level:   slog.LevelInfo,
		Message: message,
		Time:    time.Now(),
	}
}

// Error logs an error message
func (l *Logger) Error(message string) {
	l.messages <- LogMessage{
		Level:   slog.LevelError,
		Message: message,
		Time:    time.Now(),
	}
}

// Close shuts down the logger and waits for the logger goroutine to finish
func (l *Logger) Close() {
	close(l.messages)
	l.wg.Wait()
	l.file.Close()
}
