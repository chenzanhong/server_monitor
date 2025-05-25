package monitor

import (
	"backend/server/logs"
	m_init "backend/server/model/init"
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

	// 使用 GORM 查询
	var hosts []u.HostInfo
	result := m_init.DB.Table("host_info").
		Where("user_name = ? AND created_at BETWEEN ? AND ?", username, fromTime, toTime).
		Order("created_at DESC"). // 可选排序
		Find(&hosts)

	if result.Error != nil {
		log.Println(logs.GetLogPrefix(2)+"Failed to query host_info; details:", result.Error.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to query host_info",
			"details": result.Error.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"hosts": hosts})
}
