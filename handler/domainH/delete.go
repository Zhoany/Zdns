package domainH

import (
    "ZZDNS/config"
    "ZZDNS/shared"
    "log"
    "net/http"
    "strconv"

    "github.com/gin-gonic/gin"
)

func DeleteDomainRuleHandler(c *gin.Context) {
    // 1. 解析 ID
    idStr := c.Param("id")
    id, err := strconv.ParseUint(idStr, 10, 64)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
        return
    }

    db := shared.DB

    // 2. 执行删除（对 Domain 表）
    res := db.Delete(&config.Domain{}, id)
    if res.Error != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": res.Error.Error()})
        return
    }
    if res.RowsAffected == 0 {
        c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
        return
    }

    // 3. 重新加载内存中的配置
    if err := config.LoadFromDatabase(); err != nil {
        log.Printf("reload config failed after delete rule: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "deleted but reload failed"})
        return
    }

    // 4. 返回删除的 ID
    c.JSON(http.StatusOK, gin.H{"deleted_id": id})
}
