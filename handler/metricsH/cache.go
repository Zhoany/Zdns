package metricsH

import (
	"ZZDNS/logger"
	"ZZDNS/utils"
	"net/http"

	"github.com/gin-gonic/gin"
)
func MetricsCacheHitHandler(c *gin.Context) {
    start, end, ok := utils.ParseTimeRange(c)
    if !ok {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or missing time range"})
        return
    }

    logger := logger.GetLogger()

    // 直接在数据库中统计
    cacheHit, err := logger.CountLogsByEventType(c.Request.Context(), start, end, "CACHE")
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count CACHE logs: " + err.Error()})
        return
    }

    upstream, err := logger.CountLogsByEventType(c.Request.Context(), start, end, "UPSTREAM")
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count UPSTREAM logs: " + err.Error()})
        return
    }

    total := cacheHit + upstream
    var hitRate float64
    if total > 0 {
        hitRate = float64(cacheHit) / float64(total)
    }

    c.JSON(http.StatusOK, gin.H{
        "cache_hit_count":   cacheHit,
        "cache_total_count": total,
        "cache_hit_rate":    hitRate,
    })
}