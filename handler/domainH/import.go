package domainH

import (
    "bufio"
    "fmt"
    "net/http"
    "strings"

    "ZZDNS/config"
    "ZZDNS/shared"
    "github.com/gin-gonic/gin"
    "gorm.io/gorm"
    "gorm.io/gorm/clause"
)

// importResult 用于输出 JSON
type importResult struct {
    DeletedOld int64 `json:"deleted_old"`
    Imported   int64 `json:"imported"`
}

// FileImportByNameHandler 通过组名导入域名列表
func FileImportByNameHandler(c *gin.Context) {
    // 1) 必填：name + file
    name := c.PostForm("name")
    if name == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
        return
    }
    f, err := c.FormFile("file")
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
        return
    }

    // 2) 查 Group
    var group config.Group
    if err := shared.DB.Where("name = ?", name).First(&group).Error; err != nil {
        if err == gorm.ErrRecordNotFound {
            c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("group %q not found", name)})
        } else {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        }
        return
    }

    // 3) 删除旧数据
    delRes := shared.DB.Where("group_id = ?", group.ID).Delete(&config.Domain{})
    if delRes.Error != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": delRes.Error.Error()})
        return
    }
    deletedOld := delRes.RowsAffected

    // 4) 批量读取文件并写入
    in, err := f.Open()
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open file"})
        return
    }
    defer in.Close()

    scanner := bufio.NewScanner(in)
    batchSize := 2000
    var batch []config.Domain
    var imported int64

    // 跳过事务加速写入
    sess := shared.DB.Session(&gorm.Session{SkipDefaultTransaction: true})

    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        if line == "" {
            continue
        }
        batch = append(batch, config.Domain{
            Domain:  line,
            GroupID: group.ID,
        })
        if len(batch) >= batchSize {
            res := sess.
                Clauses(clause.OnConflict{
                    Columns:   []clause.Column{{Name: "domain"}},
                    DoUpdates: clause.AssignmentColumns([]string{"group_id"}),
                }).
                CreateInBatches(&batch, batchSize)
            if res.Error != nil {
                c.JSON(http.StatusInternalServerError, gin.H{"error": res.Error.Error()})
                return
            }
            imported += res.RowsAffected
            batch = batch[:0]
        }
    }
    if err := scanner.Err(); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "scan error"})
        return
    }
    if len(batch) > 0 {
        res := sess.
            Clauses(clause.OnConflict{
                Columns:   []clause.Column{{Name: "domain"}},
                DoUpdates: clause.AssignmentColumns([]string{"group_id"}),
            }).
            CreateInBatches(&batch, len(batch))
        if res.Error != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": res.Error.Error()})
            return
        }
        imported += res.RowsAffected
    }

    // 5) 重新加载内存配置
    if err := config.LoadFromDatabase(); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "reload failed: " + err.Error()})
        return
    }

    // 6) 返回结果
    c.JSON(http.StatusOK, importResult{
        DeletedOld: deletedOld,
        Imported:   imported,
    })
}
