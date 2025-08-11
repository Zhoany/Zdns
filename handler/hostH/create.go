package hostH

import (
	"ZZDNS/config"
	"ZZDNS/shared"
	"ZZDNS/utils"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type hostEntryReq struct {
	Host string `json:"host" binding:"required"`
	IP   string `json:"ip" binding:"required"`
}

func CreateHostEntry(c *gin.Context) {
	var req hostEntryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !utils.IsValidIP(req.IP) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid IP"})
		return
	}
	entry := config.HostEntry{Host: req.Host, IP: req.IP}
	db := shared.DB
	if err := db.Create(&entry).Error; err != nil {
		// 如果违反唯一约束，返回 409
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			c.JSON(http.StatusConflict, gin.H{"error": "host already exists"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	if err := config.LoadHostsFromDB(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "reload failed"})
		return
	}
	c.JSON(http.StatusCreated, entry)
}