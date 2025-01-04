package logger

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

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
}

var (
	instance *Logger
	once     sync.Once
)

// NewLogger creates a new Logger instance and starts the logging goroutine
func NewLogger(logFile string, maxSize int64, maxBackups int, bufferSize int) (*Logger, error) {
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
	}

	// 启动日志处理goroutine
	l.wg.Add(1)
	go l.processMessages()

	return l, nil
}

// initLogger initializes the global logger instance
func initLogger() {
	var err error
	instance, err = NewLogger("logs/application.log", 500*1024*1024, 7, 100) // 500MB, 7个备份
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
    for {
        select {
        case msg, ok := <-l.messages:
            if !ok {
                return // 通道已关闭，退出goroutine
            }
            // 检查文件大小并进行轮换
            l.rotateLogIfNeeded()
            // 记录日志信息
            slog.Log(ctx, msg.Level, msg.Message, "time", msg.Time)
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