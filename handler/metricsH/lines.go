// metricsH/metrics.go
package metricsH

import (
	"ZZDNS/logger"
	"ZZDNS/utils"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func MetricsRequestsHandler(c *gin.Context) {
    start, end, ok := utils.ParseTimeRange(c)
    if !ok {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or missing time range"})
        return
    }

    lg := logger.GetLogger()

    // 按秒聚合
    secBuckets, err := lg.QueryCountByBucket(c.Request.Context(), start, end, "1 second")
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    // 按分钟聚合
    minBuckets, err := lg.QueryCountByBucket(c.Request.Context(), start, end, "1 minute")
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

   sh := utils.ShanghaiLocation()

perSec := make([]gin.H, 0, len(secBuckets))
for _, b := range secBuckets {
    perSec = append(perSec, gin.H{
        // 直接用 RFC3339 格式，会包含 +08:00 偏移
        "time":  b.Bucket.In(sh).Format(time.RFC3339),
        "count": b.Count,
    })
}
perMin := make([]gin.H, 0, len(minBuckets))
for _, b := range minBuckets {
    perMin = append(perMin, gin.H{
        "time":  b.Bucket.In(sh).Format("2006-01-02T15:04:05-07:00"),
        "count": b.Count,
    })
}

c.JSON(http.StatusOK, gin.H{
    "timezone":   "Asia/Shanghai",
    "utc_offset": "+08:00",
    "per_second": perSec,
    "per_minute": perMin,
})
}

