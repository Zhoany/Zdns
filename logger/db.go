package logger

import (
	"ZZDNS/utils"
	_ "ZZDNS/utils"
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // PostgreSQL driver
	"github.com/lib/pq"
)

type BucketCount struct {
    Bucket time.Time
    Count  int
}

// QueryCountByBucket 在数据库里按给定粒度聚合
func (l *Logger) QueryCountByBucket(ctx context.Context, start, end time.Time, bucket string) ([]BucketCount, error) {
    // bucket 例: '1 second' 或 '1 minute'
    sql := fmt.Sprintf(`
        SELECT time_bucket('%s', time) AS bucket,
               count(*) AS count
        FROM logs
        WHERE time >= $1 AND time <= $2 AND level = $3
        GROUP BY bucket
        ORDER BY bucket;
    `, bucket)

    rows, err := l.db.QueryContext(ctx, sql, start.UTC(), end.UTC(), "INFO")
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var out []BucketCount

    for rows.Next() {
        var bc BucketCount
        if err := rows.Scan(&bc.Bucket, &bc.Count); err != nil {
            return nil, err
        }
        // 把 UTC 转成上海时间（用于返回给客户端）
        bc.Bucket = bc.Bucket.In(utils.ShanghaiLocation())

        out = append(out, bc)
    }
    return out, rows.Err()
}
func (l *Logger) CountLogsByEventType(ctx context.Context, start, end time.Time, eventType string) (int, error) {
   
	query := `SELECT COUNT(*) FROM logs WHERE event_type = $1 AND time BETWEEN $2 AND $3`
    var count int
    err := l.db.QueryRowContext(ctx, query, eventType, start, end).Scan(&count)
    return count, err
}

func (l *Logger) CountLogsWithDuration(ctx context.Context, start, end time.Time, level string) (int, error) {
    query := `
        SELECT COUNT(*) FROM logs
        WHERE level = $1 AND time BETWEEN $2 AND $3 AND duration_ms > 0
    `
    var count int
    err := l.db.QueryRowContext(ctx, query, level, start, end).Scan(&count)
    return count, err
}

func (l *Logger) AvgDurationWithCount(ctx context.Context, start, end time.Time, level string) (avg float64, count int, err error) {
    query := `
        SELECT AVG(duration_ms), COUNT(*)
        FROM logs
        WHERE level = $1 AND time BETWEEN $2 AND $3 AND duration_ms > 0
    `
    var avgNull sql.NullFloat64
    row := l.db.QueryRowContext(ctx, query, level, start, end)
    err = row.Scan(&avgNull, &count)
    if err != nil {
        return
    }

    if avgNull.Valid {
        avg = avgNull.Float64
    } else {
        avg = 0.0 // or leave as zero-value
    }

    return
}

func (l *Logger) CountLogsByLevel(ctx context.Context, start, end time.Time, level string) (int, error) {
    query := `
       SELECT COUNT(*) FROM logs
       WHERE level = $1 AND time BETWEEN $2 AND $3
    `
    var count int
    err := l.db.QueryRowContext(ctx, query, level, start, end).Scan(&count)
    return count, err
}

type AggregatedLog struct {
    Time      time.Time      `json:"time"`
    SourceIP  string         `json:"source_ip"`
    QueryName string         `json:"query_name"`
    EventType string         `json:"event_type"`
    Upstream  *string        `json:"upstream"`
    Duration  float64        `json:"duration_ms"`
    Answers   pq.StringArray `json:"answers"`
    IsError   bool           `json:"is_error"` // 标志是否为错误信息
}

type AggregateFilter struct {
    SourceIP  string
    QueryName string
    EventType string
    Upstream  string
}
func (l *Logger) AggregateLogs(ctx context.Context, start, end time.Time, f AggregateFilter) ([]AggregatedLog, error) {
    query := `
-- 需要 pg_trgm 扩展已启用以支持 similarity()
-- CREATE EXTENSION IF NOT EXISTS pg_trgm;

WITH filtered AS (
  SELECT *
  FROM logs
  WHERE time BETWEEN $1 AND $2
),
exact_flags AS (
  SELECT
    (COALESCE($3, '') <> '' AND EXISTS (SELECT 1 FROM filtered f WHERE f.source_ip = $3)) AS has_source_ip_exact,
    (COALESCE($4, '') <> '' AND EXISTS (SELECT 1 FROM filtered f WHERE f.query_name = $4)) AS has_query_name_exact,
    (COALESCE($5, '') <> '' AND EXISTS (SELECT 1 FROM filtered f WHERE f.event_type = $5)) AS has_event_type_exact,
    (COALESCE($6, '') <> '' AND EXISTS (SELECT 1 FROM filtered f WHERE f.upstream = $6)) AS has_upstream_exact
)
SELECT
  l.time AS time,
  l.source_ip,
  l.query_name,
  l.event_type,
  l.upstream,
  l.duration_ms,
  l.answers,
  l.error_msg
FROM logs l
CROSS JOIN exact_flags ef
WHERE l.time BETWEEN $1 AND $2
  AND (
    COALESCE($3, '') = ''
    OR (ef.has_source_ip_exact AND l.source_ip = $3)
    OR (
      NOT ef.has_source_ip_exact
      AND (
        l.source_ip ILIKE '%' || $3 || '%'
        OR similarity(l.source_ip, $3) > 0.9
      )
    )
  )
  AND (
    COALESCE($4, '') = ''
    OR (ef.has_query_name_exact AND l.query_name = $4)
    OR (
      NOT ef.has_query_name_exact
      AND (
        l.query_name ILIKE '%' || $4 || '%'
        OR similarity(l.query_name, $4) > 0.6
      )
    )
  )
  AND (
    COALESCE($5, '') = ''
    OR (ef.has_event_type_exact AND l.event_type = $5)
    OR (
      NOT ef.has_event_type_exact
      AND (
        l.event_type ILIKE '%' || $5 || '%'
        OR similarity(l.event_type, $5) > 0.9
      )
    )
  )
  AND (
    COALESCE($6, '') = ''
    OR (ef.has_upstream_exact AND l.upstream = $6)
    OR (
      NOT ef.has_upstream_exact
      AND (
        l.upstream ILIKE '%' || $6 || '%'
        OR similarity(l.upstream, $6) > 0.9
      )
    )
  )
ORDER BY l.time DESC;
`
    rows, err := l.db.QueryContext(ctx, query,
        start,
        end,
        f.SourceIP,
        f.QueryName,
        f.EventType,
        f.Upstream,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var results []AggregatedLog
  
for rows.Next() {
    var r AggregatedLog
    var errorMsg sql.NullString

    err := rows.Scan(
        &r.Time,
        &r.SourceIP,
        &r.QueryName,
        &r.EventType,
        &r.Upstream,
        &r.Duration,
        &r.Answers,
        &errorMsg,
    )
    if err != nil {
        return nil, err
    }

    // 转成上海时间再返回
    r.Time = r.Time.In(utils.ShanghaiLocation())

    if errorMsg.Valid && errorMsg.String != "" {
        r.Answers = pq.StringArray{errorMsg.String}
        r.IsError = true
    } else {
        r.IsError = false
    }

    results = append(results, r)
}



    if err = rows.Err(); err != nil {
        return nil, err
    }

    return results, nil
}
