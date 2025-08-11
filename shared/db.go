package shared

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	DB      *gorm.DB
	once    sync.Once
	initErr error
)

func getEnvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// InitDBFromEnv 初始化全局 gorm.DB（线程安全，只执行一次），日志最小只报错
func InitDBFromEnv() error {
	once.Do(func() {
		host := getEnvDefault("PGHOST", "localhost")
		port := getEnvDefault("PGPORT", "5432")
		user := getEnvDefault("PGUSER", "postgres")
		password := os.Getenv("PGPASSWORD")
		dbName := getEnvDefault("PGDATABASE", "configdb")
		sslMode := getEnvDefault("PGSSLMODE", "disable")

		dsn := fmt.Sprintf(
			"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			host, port, user, password, dbName, sslMode,
		)

		// 只保留错误日志、忽略 record not found、关闭颜色
		gormLogger := logger.New(
			log.New(os.Stderr, "", log.LstdFlags),
			logger.Config{
				SlowThreshold:             time.Second,
				LogLevel:                  logger.Error,
				IgnoreRecordNotFoundError: true,
				Colorful:                  false,
			},
		)

		DB, initErr = gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger: gormLogger,
		})
	})
	return initErr
}

// GetDB 返回已初始化的 *gorm.DB，未初始化会 panic
func GetDB() *gorm.DB {
	if DB == nil {
		panic("shared: DB not initialized, call InitDBFromEnv first")
	}
	return DB
}

// CloseDB 关闭底层连接池（程序退出时调用一次）
func CloseDB() error {
	if DB == nil {
		return nil
	}
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
