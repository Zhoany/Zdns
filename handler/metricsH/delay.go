package metricsH

import (
   
    "math"
    "net/http"

    "ZZDNS/logger"
    "ZZDNS/utils"

    "github.com/gin-gonic/gin"
)

func MetricsAvgDurationHandler(c *gin.Context) {
    start, end, ok := utils.ParseTimeRange(c)
    if !ok {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or missing time range"})
        return
    }

    avg, count, err := logger.GetLogger().AvgDurationWithCount(c.Request.Context(), start, end, "INFO")
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to compute average duration: " + err.Error()})
        return
    }

    if count == 0 {
        c.JSON(http.StatusOK, gin.H{"avg_duration_ms": 0.0, "count": 0})
        return
    }
    if avg <= 0 {
        c.JSON(http.StatusOK, gin.H{"avg_duration_ms": 0.0, "count": count})
        return
    }
    avg = math.Round(avg*100) / 100 // round to 2 decimal places

    c.JSON(http.StatusOK, gin.H{
        "avg_duration_ms": avg,
        "count":           count,
    })
}