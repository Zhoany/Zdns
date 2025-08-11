package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"ZZDNS/config"
	"ZZDNS/logger"
	"ZZDNS/server"
	"ZZDNS/shared"
)

func main() {
    // 使用所有 CPU 核心
    runtime.GOMAXPROCS(runtime.NumCPU())

    // 命令行参数：配置文件 + SQLite DSN
    var (
        cfgPath string
       // dbDSN   string
    )
    flag.StringVar(&cfgPath, "config", "./data/config.yaml", "path to YAML config file")
   // flag.StringVar(&dbDSN, "db", "./db/config.db", "SQLite DSN for storing config")
    //flag.Parse()
if err := shared.InitDBFromEnv(); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
    defer logger.GetLogger().Close()
	 var err error
	// shared.DB, err = gorm.Open(sqlite.Open(dbDSN), &gorm.Config{})
    if err != nil {
        log.Fatalf("open config DB failed: %v", err)
    }
    // 在程序退出时，关闭底层的 *sql.DB
   
    // 把 dbDSN 一并传进去
    if err := config.LoadConfig(cfgPath); err != nil {
        fmt.Println("Could not load config: %v", err)
    }

    go server.DNSServer()
    go server.ApiServer()

    // 等待退出信号
    sigs := make(chan os.Signal, 1)
    signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
    <-sigs
    
}
