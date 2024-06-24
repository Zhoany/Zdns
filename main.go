package main

import (
	"NEWzDNS/config"
	"NEWzDNS/pool"
	"NEWzDNS/rule"
	"NEWzDNS/server"
	"log"
	"runtime"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	if err := config.LoadConfig("conf/config.yaml"); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

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
