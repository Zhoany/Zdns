package logger

import (
	"context"
	"log"
	"os"
	"sync"
	"time"

	"golang.org/x/exp/slog"
)

// LogMessage represents a log message structure
type LogMessage struct {
	Level   slog.Level
	Message string
	Time    time.Time
}

// Logger represents a custom logger
type Logger struct {
	messages chan LogMessage
	wg       sync.WaitGroup
}

var (
	instance *Logger
	once     sync.Once
)

// NewLogger creates a new Logger instance and starts the logging goroutine
func NewLogger(logFile string, bufferSize int) (*Logger, error) {
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
		messages: make(chan LogMessage, bufferSize),
	}

	// 启动日志处理goroutine
	l.wg.Add(1)
	go l.processMessages()

	return l, nil
}

// initLogger initializes the global logger instance
func initLogger() {
	var err error
	instance, err = NewLogger("logs/application.log", 100)
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
	ctx := context.Background() // 使用背景上下文
	for msg := range l.messages {
		// 记录日志信息
		slog.Log(ctx, msg.Level, msg.Message, "time", msg.Time)
	}
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
}