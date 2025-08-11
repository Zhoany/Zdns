package hostH

import (
    "ZZDNS/config"
    "ZZDNS/shared"
    "log"
    "net/http"
    "strconv"

    "github.com/gin-gonic/gin"
)

// deleteHostEntryHandler 删除指定 ID 的 HostEntry
func DeleteHostEntryHandler(c *gin.Context) {
    // 1) 解析 path 参数 id
    idStr := c.Param("id")
    id, err := strconv.ParseUint(idStr, 10, 64)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
        return
    }

    db := shared.DB

    // 2) 从 host_entries 表删除
    res := db.Delete(&config.HostEntry{}, id)
    if res.Error != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": res.Error.Error()})
        return
    }
    if res.RowsAffected == 0 {
        c.JSON(http.StatusNotFound, gin.H{"error": "host entry not found"})
        return
    }

    // 3) 重新加载内存中的 hosts 映射
    if err := config.LoadHostsFromDB(); err != nil {
        log.Printf("reload config failed after deleting host entry: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "deleted but reload failed"})
        return
    }

    // 4) 返回删除的 ID
    c.JSON(http.StatusOK, gin.H{"deleted_id": id})
}
