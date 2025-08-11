package domainH

import (
    "errors"
    "net/http"

    "ZZDNS/config"
    "ZZDNS/shared"
    "github.com/gin-gonic/gin"
    "gorm.io/gorm"
)

type domainRuleReq struct {
    Name   string `json:"name" binding:"required"`
    Domain string `json:"domain" binding:"required"`
}

// CreateDomainRule 如果域名已存在，则移动到指定组；否则新建
func CreateDomainRule(c *gin.Context) {
    var req domainRuleReq
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    if req.Domain == "*" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid domain"})
        return
    }

    db := shared.DB

    // 1. 查 Group 是否存在
    var group config.Group
    if err := db.Where("name = ?", req.Name).First(&group).Error; err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            c.JSON(http.StatusBadRequest, gin.H{"error": "group not found"})
        } else {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        }
        return
    }

    // 2. 查域名是否已存在
    var existing config.Domain
    err := db.Where("domain = ?", req.Domain).First(&existing).Error
    if err == nil {
        // 已存在：移动到新组
        existing.GroupID = group.ID
        if err := db.Save(&existing).Error; err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }
        if err := config.LoadFromDatabase(); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "reload failed: " + err.Error()})
            return
        }
        c.JSON(http.StatusOK, gin.H{
            "id":       existing.ID,
            "domain":   existing.Domain,
            "moved_to": req.Name,
        })
        return
    }
    if !errors.Is(err, gorm.ErrRecordNotFound) {
        // 查询出错
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    // 3. 不存在：新建记录
    newRule := config.Domain{
        Domain:  req.Domain,
        GroupID: group.ID,
    }
    if err := db.Create(&newRule).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    if err := config.LoadFromDatabase(); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "reload failed: " + err.Error()})
        return
    }

    c.JSON(http.StatusCreated, gin.H{
        "id":     newRule.ID,
        "domain": newRule.Domain,
        "group":  req.Name,
    })
}
