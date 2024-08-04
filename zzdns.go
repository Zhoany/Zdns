package main

import (
	"ZZDNS/config"
	"ZZDNS/logger"
	"ZZDNS/server"
	"log"
	"runtime"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	
	err := config.LoadConfig("./cfg.data/cfg.yaml")
	if err != nil {
		log.Fatalf("Could not load config: %v", err)
	}
	server.StartServer()
	defer logger.GetLogger().Close()
}
