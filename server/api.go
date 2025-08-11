// 提供仅批量操作的 API 服务，含按 name 混合新增/更新端点。
package server

import (
	"fmt"
	"log"

	"net/http"

	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	// 替换为实际 import 路径
	"ZZDNS/config"
	"ZZDNS/handler/domainH"
	"ZZDNS/handler/groupH"
	"ZZDNS/handler/hostH"
	"ZZDNS/handler/loginH"
	"ZZDNS/handler/metricsH"

	"ZZDNS/shared"
)

func ApiServer() {
	if err := shared.InitDBFromEnv(); err != nil {
	log.Fatalf("初始化数据库失败: %v", err)
}
	if err := startApiServer(); err != nil {
		log.Fatalf("API server exited: %v", err)
	}
}

func startApiServer() error {
	r := gin.Default()
	// === 在这里注册 CORS 中间件 ===
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"}, // 允许的域名列表，推荐替换成你前端所在的真实域名
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	r.POST("/api/login", loginH.LoginHandler)

	api := r.Group("/api", loginH.JwtAuthMiddleware())
	{
		// 查询
		api.GET("/listDomainRules", domainH.ListDomainRules) // ?offset=&limit=
		api.GET("/listHostEntries", hostH.ListHostEntries)
		api.GET("/listDomainRuleCounts", domainH.ListDomainRuleCounts)
		api.GET("/listDomainRulesByName", domainH.ListDomainRulesByName)
		api.GET("/searchDomainRule", domainH.SearchDomainRule)
		//文件导入
		api.POST("/importDomainRulesByName", domainH.FileImportByNameHandler)
		api.POST("/importHostEntries", hostH.FileImportHostEntriesHandler)
		//删除组
		api.POST("/deleteGroupByName", groupH.DeleteGroupByName)
		api.GET("/ListDomainRuleGroups", groupH.ListDomainRuleGroupsHandler)
		// 单个删除
		api.DELETE("/hostEntries/:id", hostH.DeleteHostEntryHandler)
		api.DELETE("/domainRules/:id", domainH.DeleteDomainRuleHandler)

		// 新建条目
		api.POST("/hostEntries", hostH.CreateHostEntry)
		api.POST("/domainRules", domainH.CreateDomainRule)

		// 修改条目
		api.PUT("/hostEntries/:id", hostH.UpdateHostEntryHandler)
		api.PUT("/domainRules/:id", domainH.UpdateDomainRuleHandler)
		//新建域名分组并上传文件
		api.POST("/domainRuleGroups", groupH.CreateDomainRuleGroup)
		api.PUT("/groups/:name", groupH.UpdateGroupHandler)
		//日志和监控
		api.GET("/search/logs", metricsH.MetricsAggregateHandler)
		api.GET("/metrics/requests", metricsH.MetricsRequestsHandler)
		api.GET("/metrics/error_count", metricsH.MetricsErrorCountHandler)
		api.GET("/metrics/avg_duration", metricsH.MetricsAvgDurationHandler)
		api.GET("/metrics/counts", metricsH.MetricsCountHandler)
		api.GET("/metrics/cachehit", metricsH.MetricsCacheHitHandler)
		// 显式 reload
		api.POST("/reload", func(c *gin.Context) {
			if err := config.LoadFromDatabase(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"status": "err", "error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})
	}

	addr := fmt.Sprintf(":%d", config.CFG.API.Port)
	log.Printf("API 服务已启动, 监听 %s", addr)
	return r.Run(addr)
}
