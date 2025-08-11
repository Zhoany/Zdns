package metricsH

import (
    "net/http"

    "ZZDNS/logger"
    "ZZDNS/utils"

    "github.com/gin-gonic/gin"
)

func MetricsAggregateHandler(c *gin.Context) {
    // 时间范围解析
    start, end, ok := utils.ParseTimeRange(c)
    
    if !ok {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or missing time range"})
        return
    }

    // 可选模糊过滤条件
    filter := logger.AggregateFilter{
        SourceIP:  c.Query("source_ip"),
        QueryName: c.Query("query_name"),
        EventType: c.Query("event_type"),
        Upstream:  c.Query("upstream"),
    }

    // 查询聚合结果
    results, err := logger.GetLogger().AggregateLogs(c.Request.Context(), start, end, filter)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "aggregation failed: " + err.Error()})
        return
    }

    c.JSON(http.StatusOK, results)
}
