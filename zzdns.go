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

	cfgPath := "./cfg.data/cfg.yaml"
	err := config.LoadConfig(cfgPath)
	if err != nil {
		log.Fatalf("Could not load config: %v", err)
	}
	server.StartServer(cfgPath)
	defer logger.GetLogger().Close()
}
