package groupH

import (
    "ZZDNS/config"
    "ZZDNS/shared"
    "log"
    "net/http"

    "github.com/gin-gonic/gin"
)

type deleteByNameReq struct {
    Name string `json:"name" binding:"required"`
}

func DeleteGroupByName(c *gin.Context) {
    var req deleteByNameReq
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // 禁止删除默认组
    if req.Name == "default" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "cannot delete default group"})
        return
    }

    db := shared.DB

    // 删除 Group，依赖 GORM 的 OnDelete:CASCADE 会自动删除关联的 Domain
    res := db.Where("name = ?", req.Name).Delete(&config.Group{})
    if res.Error != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": res.Error.Error()})
        return
    }
    if res.RowsAffected == 0 {
        c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
        return
    }

    // 重新加载到内存
    if err := config.LoadFromDatabase(); err != nil {
        log.Printf("reload config failed after delete: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "deleted but reload failed"})
        return
    }

    // 返回删除数量
    c.JSON(http.StatusOK, gin.H{"deleted": res.RowsAffected})
}
