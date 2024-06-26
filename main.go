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

	
	fmt.Printf("DNS Server is running on port: %v\n", config.Cfg.Server.Address)

	
	maxConcurrency := config.Cfg.Server.MaxClients
	sem := make(chan struct{}, maxConcurrency)

	
	pool.InitPool(config.Cfg.Server.MaxWorkers, config.Cfg.Server.MaxConnects)
	defer pool.Release()

	rule.InitDomainMatcher()
	rule.LoadUpstreamRules()
	rule.LoadBlocklist()

	go server.StartDNSServer(sem) 

	select {}
}
