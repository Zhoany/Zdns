package groupH

import (
    "net/http"

    "ZZDNS/config"
    "ZZDNS/shared"
    "ZZDNS/utils"

    "github.com/gin-gonic/gin"
    "gorm.io/gorm"
)

type updateGroupReq struct {
    Upstream string `json:"upstream,omitempty"`  // 新的上游地址
    V6       *bool  `json:"v6,omitempty"`        // 是否支持 IPv6
    Priority *int   `json:"priority,omitempty"`  // 优先级
}

// UpdateGroupHandler 更新指定组的配置（upstream、v6、priority）
//
// 路由示例：
//   api.PUT("/groups/:name", groupH.UpdateGroupHandler)
func UpdateGroupHandler(c *gin.Context) {
    // 1) 从 URL 中取组名
    name := c.Param("name")
    if name == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "group name is required"})
        return
    }

    // 2) 绑定请求体
    var req updateGroupReq
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // 3) 如果提供了 upstream，就校验它
    if req.Upstream != "" && !utils.IsValidUpstream(req.Upstream) {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid upstream"})
        return
    }

    db := shared.DB

    // 4) 读取现有组
    var grp config.Group
    if err := db.Where("name = ?", name).First(&grp).Error; err != nil {
        if err == gorm.ErrRecordNotFound {
            c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
        } else {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        }
        return
    }

    // 5) 准备更新字段
    updates := make(map[string]interface{})
    if req.Upstream != "" {
        updates["upstream"] = req.Upstream
    }
    if req.V6 != nil {
        updates["v6"] = *req.V6
    }
    if req.Priority != nil {
        updates["priority"] = *req.Priority
    }

    if len(updates) == 0 {
        c.JSON(http.StatusBadRequest, gin.H{"error": "no fields to update"})
        return
    }

    // 6) 执行更新
    if err := db.Model(&grp).Updates(updates).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    // 7) 重新加载内存配置
    if err := config.LoadFromDatabase(); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "reload failed: " + err.Error()})
        return
    }

    // 8) 返回最新的配置信息
    c.JSON(http.StatusOK, gin.H{
        "name":     grp.Name,
        "upstream": grp.Upstream,
        "v6":       grp.V6,
        "priority": grp.Priority,
    })
}
