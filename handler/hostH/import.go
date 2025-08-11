package hostH

import (
	"ZZDNS/config"
	"ZZDNS/shared"
	"bufio"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)
func FileImportHostEntriesHandler(c *gin.Context) {
	// 1) 读取文件
	f, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	in, err := f.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer in.Close()

	// 2) 扫描每行：假设格式 "host IP"，空格或制表符分隔
	scanner := bufio.NewScanner(in)
	const batchSize = 2000
	var (
		batch   []config.HostEntry
		created int64

		skipped int64
	)
	db:=shared.DB
	sess := db.Session(&gorm.Session{SkipDefaultTransaction: true})
	for scanner.Scan() {
		line := strings.Fields(scanner.Text())
		if len(line) != 2 {
			skipped++
			continue
		}
		host, ip := line[0], line[1]
		if net.ParseIP(ip) == nil {
			skipped++
			continue
		}
		batch = append(batch, config.HostEntry{Host: host, IP: ip})

		if len(batch) >= batchSize {
			res := sess.
				Clauses(clause.OnConflict{
					Columns:   []clause.Column{{Name: "host"}},
					DoUpdates: clause.AssignmentColumns([]string{"ip"}),
				}).
				CreateInBatches(&batch, batchSize)
			if res.Error != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": res.Error.Error()})
				return
			}
			// RowsAffected = 插入 + 更新
			// 假设更新占比小，可以把它们都算到 updated
			created += res.RowsAffected
			batch = batch[:0]
		}
	}
	if err := scanner.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(batch) > 0 {
		res := sess.
			Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "host"}},
				DoUpdates: clause.AssignmentColumns([]string{"ip"}),
			}).
			CreateInBatches(&batch, len(batch))
		if res.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": res.Error.Error()})
			return
		}
		created += res.RowsAffected
	}

	// 3) 如果你在内存里也会缓存 HostEntry，可以在这里 reload
	if err := config.LoadHostsFromDB(); err != nil {
		log.Printf("reload config failed: %v", err)
	}

	// 4) 返回结果
	c.JSON(http.StatusOK, gin.H{
		"imported": created,
		"skipped":  skipped,
	})
}