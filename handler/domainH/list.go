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

// domainRuleResp 是为前端封装的单条输出结构
type domainRuleResp struct {
	ID     uint   `json:"id"`
	Domain string `json:"domain"`
	Group  string `json:"group"`
}

// listDomainRules 支持 offset/limit 分页，并返回关联的 Group 信息
func ListDomainRules(c *gin.Context) {
	// 1. 解析分页参数
	offset := 0
	limit := 500
	if v := c.Query("offset"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			offset = i
		}
	}
	if v := c.Query("limit"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			limit = i
		}
	}

	// 2. 先拿总数
	var total int64
	if err := shared.DB.Model(&config.Domain{}).Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 3. 分页查询并预加载 Group 关联
	var domains []config.Domain
	if err := shared.DB.
		Preload("Group").
		Offset(offset).
		Limit(limit).
		Order("id ASC").
		Find(&domains).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 4. 构造返回的 items 列表
	resp := make([]domainRuleResp, 0, len(domains))
	for _, d := range domains {
		resp = append(resp, domainRuleResp{
			ID:     d.ID,
			Domain: d.Domain,
			Group:  d.Group.Name,
		})
	}

	// 5. 返回 JSON
	c.JSON(http.StatusOK, gin.H{
		"total": total,
		"items": resp,
	})
}

// listByNameResp 定义单条返回结构
type listByNameResp struct {
	ID       uint   `json:"id"`
	Domain   string `json:"domain"`
	Upstream string `json:"upstream"`
	V6       bool   `json:"v6"`
	Priority int    `json:"priority"`
}

func ListDomainRulesByName(c *gin.Context) {
	name := c.Query("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing name"})
		return
	}

	// 1. 找组
	var group config.Group
	if err := shared.DB.
		Where("name = ?", name).
		First(&group).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	// 2. 统计总数
	var total int64
	if err := shared.DB.
		Model(&config.Domain{}).
		Where("group_id = ?", group.ID).
		Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 如果太多就只报数不展示
	if total >= 250 {
		c.JSON(http.StatusOK, gin.H{
			"total": total,
			"msg":   "域名过多不予展示",
		})
		return
	}

	// 3. 拉取列表并预加载 Group
	var domains []config.Domain
	if err := shared.DB.
		Preload("Group").
		Where("group_id = ?", group.ID).
		Order("id ASC").
		Find(&domains).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 4. 构造返回结构
	items := make([]listByNameResp, 0, len(domains))
	for _, d := range domains {
		items = append(items, listByNameResp{
			ID:       d.ID,
			Domain:   d.Domain,
			Upstream: d.Group.Upstream,
			V6:       d.Group.V6,
			Priority: d.Group.Priority,
		})
	}

	// 5. 返回 JSON
	c.JSON(http.StatusOK, gin.H{
		"total": total,
		"items": items,
	})
}

type nameCount struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

func ListDomainRuleCounts(c *gin.Context) {
	var counts []nameCount

	// 从 groups 表出发，左连接 domains，确保所有组都会出现
	if err := shared.DB.
		Model(&config.Group{}).
		Select("groups.name AS name, COUNT(domains.id) AS count").
		Joins("LEFT JOIN domains ON domains.group_id = groups.id").
		Group("groups.name").
		Scan(&counts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"counts": counts})
}
