package hostH

import (
	"ZZDNS/config"
	"ZZDNS/shared"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

 func ListHostEntries(c *gin.Context) {
	offset := 0
	limit := 500
	if v := c.Query("offset"); v != "" {
		fmt.Sscan(v, &offset)
	}
	if v := c.Query("limit"); v != "" {
		fmt.Sscan(v, &limit)
	}

	var hosts []config.HostEntry
	if err := shared.DB.Offset(offset).Limit(limit).Find(&hosts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"total": len(hosts), "items": hosts})
}
