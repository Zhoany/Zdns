package metricsH

import (
    "net/http"

    "ZZDNS/logger"
    "ZZDNS/utils"

    "github.com/gin-gonic/gin"
)

func MetricsErrorCountHandler(c *gin.Context) {
    // 1. 解析时间范围
    start, end, ok := utils.ParseTimeRange(c)
    if !ok {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or missing time range"})
        return
    }

    // 2. 直接在数据库中统计 ERROR 日志数量
    count, err := logger.GetLogger().CountLogsByLevel(c.Request.Context(), start, end, "ERROR")
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    // 3. 返回结果
    c.JSON(http.StatusOK, gin.H{"error_count": count})
}
