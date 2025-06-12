package monitor

import (
	m_init "backend/server/model/init"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// WarningRecord 定义告警记录结构体
type WarningRecord struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	HostName     string    `json:"host_name"`
	WarningType  string    `json:"warning_type"`
	WarningTitle string    `json:"warning_title"`
	WarningTime  time.Time `json:"warning_time"`
}

// GetWarningRecordsByHostname 获取指定 hostname 的所有告警记录
func GetWarningRecordsByHostname(c *gin.Context) {
	hostname := c.Query("hostname")
	if hostname == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "hostname 参数不能为空",
		})
		return
	}

	var records []WarningRecord
	result := m_init.DB.Where("host_name = ?", hostname).Order("warning_time DESC").Find(&records)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "查询告警记录失败：" + result.Error.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": records,
	})
}
