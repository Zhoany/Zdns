package hostH

import (
    "net/http"
    "strconv"

    "ZZDNS/config"
    "ZZDNS/shared"
    "ZZDNS/utils"
    "github.com/gin-gonic/gin"
)



func UpdateHostEntryHandler(c *gin.Context) {
    // 1) 解析 path 参数 id
    idStr := c.Param("id")
    id, err := strconv.ParseUint(idStr, 10, 64)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
        return
    }

    // 2) 绑定并校验请求体
    var req hostEntryReq
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    if !utils.IsValidIP(req.IP) {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid IP"})
        return
    }
    // 如果你也想校验 Host 格式，可以加一个 utils.IsValidHost
    // if !utils.IsValidHost(req.Host) {
    //     c.JSON(http.StatusBadRequest, gin.H{"error": "invalid host"})
    //     return
    // }

    db := shared.DB

    // 3) 执行更新：根据 id 同时更新 Host 和 IP
    res := db.Model(&config.HostEntry{}).
        Where("id = ?", id).
        Updates(map[string]interface{}{
            "host": req.Host,
            "ip":   req.IP,
        })
    if res.Error != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": res.Error.Error()})
        return
    }
    if res.RowsAffected == 0 {
        c.JSON(http.StatusNotFound, gin.H{"error": "host entry not found"})
        return
    }

    // 4) 重新加载内存中的 hosts
    if err := config.LoadHostsFromDB(); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "reload failed: " + err.Error()})
        return
    }

    // 5) 返回更新后的记录
    c.JSON(http.StatusOK, gin.H{
        "id":   id,
        "host": req.Host,
        "ip":   req.IP,
    })
}
