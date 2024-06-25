package main

import (
	"NEWzDNS/config"
	"NEWzDNS/log"
	"NEWzDNS/pool"
	"NEWzDNS/rule"
	"NEWzDNS/server"
	"fmt"
	"runtime"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	// 加载配置
	if err := config.LoadConfig("conf/config.yaml"); err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		return
	}

	err := log.InitLogger()
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		return
	}
	defer log.Sync()

	// 显示服务器端口号
	fmt.Printf("DNS Server is running on port: %v\n", config.Cfg.Server.Address)

	// 设置最大并发连接数
	maxConcurrency := config.Cfg.Server.MaxClients
	sem := make(chan struct{}, maxConcurrency)

	// 初始化线程池和客户端池
	pool.InitPool(config.Cfg.Server.MaxWorkers, config.Cfg.Server.MaxConnects)
	defer pool.Release()

	rule.InitDomainMatcher()
	rule.LoadUpstreamRules()
	rule.LoadBlocklist()

	go server.StartDNSServer(sem) // 传递管道给 StartDNSServer

	select {}
}
