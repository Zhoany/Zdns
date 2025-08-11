package logger

import (
	"context"
	"database/sql"
	
	"log"
	"os"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // PostgreSQL driver
	"github.com/lib/pq"
)

// LogMessage represents a structured log entry with fields mapped to table columns
type LogMessage struct {
    Time       time.Time
    Level      string
    SourceIP   string
    QueryName  string
    EventType  string
    DurationMS int64
    Answers    []string
    Upstream   sql.NullString
    Rcode      sql.NullInt32
    ErrorMsg   sql.NullString
}

// Logger handles asynchronous insertion of LogMessage entries into TimescaleDB
type Logger struct {
    messages chan LogMessage
    wg       sync.WaitGroup
    db       *sql.DB
    stmt     *sql.Stmt
    shutdown sync.Once
    ctx      context.Context 
}

var (
    instance *Logger
    once     sync.Once
)

const (
    insertSQL = `
INSERT INTO logs
  (time, level, source_ip, query_name, event_type, duration_ms,
   answers, upstream, rcode, error_msg)
VALUES
  ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10);
`
    querySQL = `
SELECT
    time, level, source_ip, query_name, event_type, duration_ms,
    answers, upstream, rcode, error_msg
FROM
    logs
WHERE
    time >= $1
    AND time <= $2
    AND level = $3;
`
)

func NewLoggerPG(ctx context.Context, dsn string, bufferSize int) (*Logger, error) {
    db, err := sql.Open("pgx", dsn)
    if err != nil {
        return nil, err
    }

    testCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    if err := db.PingContext(testCtx); err != nil {
        db.Close()
        return nil, err
    }

    stmt, err := db.PrepareContext(ctx, insertSQL)
    if err != nil {
        db.Close()
        return nil, err
    }

    l := &Logger{
        messages: make(chan LogMessage, bufferSize),
        db:       db,
        stmt:     stmt,
        ctx:      ctx, // 保存
    }
    l.wg.Add(1)
    go l.processMessages()
    return l, nil
}

// init reads DSN from PG_DSN env and initializes singleton
func init() {
    dsn := os.Getenv("PG_DSN")
    if dsn == "" {
        log.Fatalf("PG_DSN environment variable is not set")
    }
    var err error
   once.Do(func() {
    instance, err = NewLoggerPG(context.Background(), dsn, 1000)
})
    if err != nil {
        log.Fatalf("failed to initialize logger: %v", err)
    }
    
  
}

// GetLogger returns the singleton Logger instance
func GetLogger() *Logger {
    return instance
}

// processMessages consumes the channel and inserts records into the logs table
func (l *Logger) processMessages() {
    defer l.wg.Done()
    for msg := range l.messages {
        writeCtx, cancel := context.WithTimeout(l.ctx, 2*time.Second)
        _, err := l.stmt.ExecContext(writeCtx,
            msg.Time,
            msg.Level,
            msg.SourceIP,
            msg.QueryName,
            msg.EventType,
            msg.DurationMS,
            pq.Array(msg.Answers),
            msg.Upstream,
            msg.Rcode,
            msg.ErrorMsg,
        )
        cancel()

        if err != nil {
            log.Printf("[Logger] write error: %v", err)
        }
    }
}

// InfoCache logs a cache hit event
func (l *Logger) InfoCache(srcIP, query string, duration time.Duration, answers []string) {
    l.messages <- LogMessage{
        Time:       time.Now(),
        Level:      "INFO",
        SourceIP:   srcIP,
        QueryName:  query,
        EventType:  "CACHE",
        DurationMS: duration.Milliseconds(),
        Answers:    answers,
        // Upstream, Rcode, ErrorMsg 均为 Null（零值），数据库若允许则插入 NULL
    }
}

// InfoUpstream logs an upstream query event
func (l *Logger) InfoUpstream(srcIP, query, upstream string, rcode int, duration time.Duration, answers []string) {
    l.messages <- LogMessage{
        Time:       time.Now(),
        Level:      "INFO",
        SourceIP:   srcIP,
        QueryName:  query,
        EventType:  "UPSTREAM",
        DurationMS: duration.Milliseconds(),
        Answers:    answers,
        Upstream:   sql.NullString{String: upstream, Valid: true},
        Rcode:      sql.NullInt32{Int32: int32(rcode), Valid: true},
    }
}

// ErrorLog logs an error event with context
func (l *Logger) ErrorLog(srcIP, query, eventType string, err error) {
    l.messages <- LogMessage{
        Time:      time.Now(),
        Level:     "ERROR",
        SourceIP:  srcIP,
        QueryName: query,
        EventType: eventType,
        // DurationMS, Answers 保持默认零值
        ErrorMsg: sql.NullString{String: err.Error(), Valid: true},
    }
}

// Close gracefully shuts down the logger, flushing any pending messages
func (l *Logger) Close() {
    l.shutdown.Do(func() {
        close(l.messages)
        l.wg.Wait()
        if l.stmt != nil {
            l.stmt.Close()
        }
        l.db.Close()
    })
}
