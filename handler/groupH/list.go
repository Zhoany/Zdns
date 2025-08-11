package groupH

import (
	"net/http"

	"ZZDNS/config"
	"ZZDNS/shared"
	"github.com/gin-gonic/gin"
)

// groupInfo 用于返回给前端的组信息结构体
// 根据需要可扩展字段，例如域名数量等
type groupInfo struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	Upstream string `json:"upstream"`
	V6       bool   `json:"v6"`
	Priority int    `json:"priority"`
}

// ListDomainRuleGroupsHandler 列出所有域名规则组信息
// GET /api/groups
func ListDomainRuleGroupsHandler(c *gin.Context) {
	db := shared.DB
	// 按优先级和名称排序，也可根据需求调整
	var groups []config.Group
	if err := db.Order("priority DESC, name ASC").Find(&groups).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 将数据库模型转换为前端响应结构
	resp := make([]groupInfo, 0, len(groups))
	for _, g := range groups {
		resp = append(resp, groupInfo{
			ID:       g.ID,
			Name:     g.Name,
			Upstream: g.Upstream,
			V6:       g.V6,
			Priority: g.Priority,
		})
	}

	c.JSON(http.StatusOK, gin.H{"groups": resp})
}
