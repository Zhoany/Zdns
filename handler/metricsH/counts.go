package metricsH

import (
    "net/http"

    "ZZDNS/logger"
    "ZZDNS/utils"

    "github.com/gin-gonic/gin"
)

func MetricsCountHandler(c *gin.Context) {
    start, end, ok := utils.ParseTimeRange(c)
    if !ok {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or missing time range"})
        return
    }

    count, err := logger.GetLogger().CountLogsWithDuration(c.Request.Context(), start, end, "INFO")
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count logs: " + err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"total_count": count})
}