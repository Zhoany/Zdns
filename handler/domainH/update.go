package domainH

import (
    "errors"
    "net/http"
    "strconv"

    "ZZDNS/config"
    "ZZDNS/shared"
    "github.com/gin-gonic/gin"
    "gorm.io/gorm"
)

// updateDomainReq 接收更新时的请求体
type updateDomainReq struct {
    Name   string `json:"name" binding:"required"`
    Domain string `json:"domain" binding:"required"`
}

func UpdateDomainRuleHandler(c *gin.Context) {
    // 1. 解析 URL 中的 ID
    idStr := c.Param("id")
    id, err := strconv.ParseUint(idStr, 10, 64)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
        return
    }

    db := shared.DB

    // 2. 读取现有记录
    var orig config.Domain
    if err := db.First(&orig, id).Error; err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
        } else {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        }
        return
    }

    // 3. 绑定并校验请求体
    var req updateDomainReq
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    if req.Domain == "*" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid domain"})
        return
    }

    // 4. 检查目标组是否存在
    var group config.Group
    if err := db.Where("name = ?", req.Name).First(&group).Error; err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            c.JSON(http.StatusBadRequest, gin.H{"error": "group not found"})
        } else {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        }
        return
    }

    // 5. 检查同名域名冲突（排除自身）
    var conflict config.Domain
    err = db.
        Where("domain = ? AND id <> ?", req.Domain, id).
        First(&conflict).Error
    if err == nil {
        c.JSON(http.StatusConflict, gin.H{"error": "another rule with same domain exists"})
        return
    } else if !errors.Is(err, gorm.ErrRecordNotFound) {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    // 6. 执行更新：域名 + 组ID
    res := db.Model(&config.Domain{}).
        Where("id = ?", id).
        Updates(map[string]interface{}{
            "domain":   req.Domain,
            "group_id": group.ID,
        })
    if res.Error != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": res.Error.Error()})
        return
    }
    if res.RowsAffected == 0 {
        c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
        return
    }

    // 7. 重新加载内存配置
    if err := config.LoadFromDatabase(); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "reload failed: " + err.Error()})
        return
    }

    // 8. 返回更新结果
    c.JSON(http.StatusOK, gin.H{
        "id":     id,
        "domain": req.Domain,
        "group":  req.Name,
    })
}
