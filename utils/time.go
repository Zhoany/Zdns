package utils

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)
func ParseTimeRange(c *gin.Context) (time.Time, time.Time, bool) {
    var (
        start time.Time
        end   time.Time
        err   error
    )
    startStr := c.Query("start")
    endStr   := c.Query("end")

    if startStr != "" {
        // 支持纳秒精度的 RFC3339
        start, err = time.Parse(time.RFC3339Nano, startStr)
        if err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "invalid start time (expect RFC3339Nano)"})
            return start, end, false
        }
    } else {
        start = time.Now().Add(-1 * time.Hour)
    }

    if endStr != "" {
        end, err = time.Parse(time.RFC3339Nano, endStr)
        if err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "invalid end time (expect RFC3339Nano)"})
            return start, end, false
        }
    } else {
        end = time.Now()
    }

    // 统一使用 UTC 时间，避免时区偏差
    return start.UTC(), end.UTC(), true
}
var shanghaiLocation *time.Location

func ShanghaiLocation() *time.Location {
    if shanghaiLocation == nil {
        var err error
        shanghaiLocation, err = time.LoadLocation("Asia/Shanghai")
        if err != nil {
            // 兜底：如果加载失败，用固定 +8 小时（虽然极少会失败）
            shanghaiLocation = time.FixedZone("CST", 8*3600)
        }
    }
    return shanghaiLocation
}