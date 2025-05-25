package monitor

import (
	"backend/server/logs"
	"backend/server/model"
	u "backend/server/model/user"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

// ListAgent 用于查询所有主机信息
func ListAgent(c *gin.Context) {
	// 从上下文中获取用户名
	Username, exists := c.Get("username")
	if !exists {
		log.Printf("未找到用户名")
		c.JSON(401, gin.H{
			"code":    401,
			"success": false,
			"message": "未找到用户信息",
		})
		return
	}
	username := Username.(string)

	// 解析时间查询参数
	from := c.Query("from")
	to := c.Query("to")

	if from == "" {
		from = "1970-01-01T00:00:00Z"
	}
	if to == "" {
		to = "9999-12-31T23:59:59Z"
	}

	fromTime, err := time.Parse(time.RFC3339, from)
	if err != nil {
		log.Println(logs.GetLogPrefix(2) + "无效的 from 时间格式")
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 from 时间格式"})
		return
	}
	toTime, err := time.Parse(time.RFC3339, to)
	if err != nil {
		log.Println(logs.GetLogPrefix(2) + "无效的 to 时间格式")
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 to 时间格式"})
		return
	}

	// 查询数据库，返回所有字段
	query := `
		SELECT id, user_name, host_name, ip, os, platform, kernel_arch, created_at, company_id
		FROM host_info
		WHERE user_name = $1 AND created_at BETWEEN $2 AND $3
	`

	rows, err := model.DB.Query(query, username, fromTime, toTime)
	if err != nil {
		log.Println(logs.GetLogPrefix(2)+"Failed to query host_info; details:", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query host_info", "details": err.Error()})
		return
	}
	defer rows.Close()

	var hosts []u.HostInfo
	for rows.Next() {
		var host u.HostInfo
		// 注意：如果字段允许为 NULL（比如 company_id 是可空的），需要使用 sql.NullInt64 等类型处理
		if err := rows.Scan(
			&host.ID,
			&host.UserName,
			&host.Hostname,
			&host.IP,
			&host.OS,
			&host.Platform,
			&host.KernelArch,
			&host.CreatedAt,
			&host.CompanyID,
		); err != nil {
			log.Println(logs.GetLogPrefix(2)+"Failed to scan host_info; details:", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan host_info", "details": err.Error()})
			return
		}
		hosts = append(hosts, host)
	}
	if err := rows.Err(); err != nil {
		log.Println(logs.GetLogPrefix(2)+"Error occurred during iteration; details:", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error occurred during iteration", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"hosts": hosts})
}
