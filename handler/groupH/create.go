package groupH

import (

	"net/http"



	"ZZDNS/config"
	"ZZDNS/shared"
	"ZZDNS/utils"

	"github.com/gin-gonic/gin"
	
)
type groupReq struct {
    Name     string `json:"name"     binding:"required"`
    Upstream string `json:"upstream" binding:"required"`
    V6       *bool  `json:"v6"       binding:"required"` // 用指针才能通过 required
    Priority int    `json:"priority"`                    // 可选
}




func CreateDomainRuleGroup(c *gin.Context) {
    var req groupReq
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // 必传但允许 false
    if req.V6 == nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "v6 is required"})
        return
    }

    if !utils.IsValidUpstream(req.Upstream) {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid upstream"})
        return
    }

    // 默认 priority
    if req.Priority == 0 {
        req.Priority = 0
    }

    db := shared.DB

    var exists int64
    if err := db.Model(&config.Group{}).Where("name = ?", req.Name).Count(&exists).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    if exists > 0 {
        c.JSON(http.StatusConflict, gin.H{"error": "group name already exists"})
        return
    }

    newGroup := config.Group{
        Name:     req.Name,
        Upstream: req.Upstream,
        V6:       *req.V6,
        Priority: req.Priority,
    }
    if err := db.Create(&newGroup).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    if err := config.LoadFromDatabase(); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "reload failed: " + err.Error()})
        return
    }

    c.JSON(http.StatusCreated, gin.H{
        "id":       newGroup.ID,
        "name":     newGroup.Name,
        "upstream": newGroup.Upstream,
        "v6":       newGroup.V6,
        "priority": newGroup.Priority,
    })
}