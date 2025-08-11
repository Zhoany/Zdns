package domainH

import (
	
	"net/http"
	"strings"

	"ZZDNS/config"
	"ZZDNS/shared"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// searchResp 用于返回匹配来源与内容
type searchResp struct {
	Domain      string `json:"domain"`                 // 查询的域名
	Source      string `json:"source"`                 // host | rule | default | none
	MatchedRule string `json:"matched_rule,omitempty"` // 命中的域名规则（如 foo.bar.com）
	Group       string `json:"group,omitempty"`        // 所属组名
	Upstream    string `json:"upstream,omitempty"`     // 上游地址
	V6          bool   `json:"v6,omitempty"`
	Priority    int    `json:"priority,omitempty"`
	IP          string `json:"ip,omitempty"`           // 若命中 hosts
}

// SearchDomainRule: GET /domain/search?domain=xxx
func SearchDomainRule(c *gin.Context) {
	d := strings.TrimSpace(c.Query("domain"))
	if d == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing domain"})
		return
	}
	ld := strings.ToLower(d)
	
	// 1. 先查 hosts
	if ip, ok := config.Hosts[ld]; ok {
		c.JSON(http.StatusOK, searchResp{
			Domain: d,
			Source: "host",
			IP:     ip,
		})
		return
	}

	// 2. 精确查表，看是否有 exact rule
	var dom config.Domain
	err := shared.DB.Preload("Group").Where("domain = ?", ld).First(&dom).Error
	if err == nil {
		c.JSON(http.StatusOK, searchResp{
			Domain:      d,
			Source:      "rule",
			MatchedRule: dom.Domain,
			Group:       dom.Group.Name,
			Upstream:    dom.Group.Upstream,
			V6:          dom.Group.V6,
			Priority:    dom.Group.Priority,
		})
		return
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 3. Trie 后缀匹配（你的 DomainTrie 只存 upstream，所以我们再反查 group）
	if up, ok := config.DomainTrie.MatchDomain(ld); ok {
		// 3.1 反查 group 信息（按 upstream & priority 最优）
		var grp config.Group
		if err := shared.DB.
			Where("upstream = ?", up).
			Order("priority ASC").
			First(&grp).Error; err != nil {
			// 找不到 group 也不致命，直接返回 upstream
			c.JSON(http.StatusOK, searchResp{
				Domain:   d,
				Source:   "rule",
				Upstream: up,
			})
			return
		}

		// 3.2 为了给出命中的域名规则，可做一次最长后缀匹配 DB 查询
		matchedRule := findLongestRuleInDB(ld, grp.ID)

		c.JSON(http.StatusOK, searchResp{
			Domain:      d,
			Source:      "rule",
			MatchedRule: matchedRule,
			Group:       grp.Name,
			Upstream:    grp.Upstream,
			V6:          grp.V6,
			Priority:    grp.Priority,
		})
		return
	}

	// 4. 都没命中，返回默认
	if config.CFG != nil && config.CFG.Server.DefaultServer != "" {
		c.JSON(http.StatusOK, searchResp{
			Domain:   d,
			Source:   "default",
			Upstream: config.CFG.Server.DefaultServer,
			V6:       config.CFG.Server.V6,
		})
		return
	}

	// 5. 真的没有
	c.JSON(http.StatusOK, searchResp{
		Domain: d,
		Source: "none",
	})
}

// findLongestRuleInDB 尝试在当前组里找与 domain 最长后缀匹配的规则，
// 仅用于展示 matched_rule，可选实现，不影响主逻辑。
func findLongestRuleInDB(domain string, groupID uint) string {
	labels := strings.Split(domain, ".")
	for i := 0; i < len(labels); i++ {
		candidate := strings.Join(labels[i:], ".")
		var d config.Domain
		err := shared.DB.
			Where("group_id = ? AND domain = ?", groupID, candidate).
			First(&d).Error
		if err == nil {
			return d.Domain
		}
	}
	return ""
}
