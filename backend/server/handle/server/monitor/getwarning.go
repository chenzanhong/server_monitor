package monitor

import (
	m_init "backend/server/model/init"
	"net/http"
	u "backend/server/model/user"

	"github.com/gin-gonic/gin"
)

// GetWarningRecordsByHostname 获取指定 hostname 的所有告警记录
func GetWarningRecordsByHostname(c *gin.Context) {

	hostname := c.Param("hostname")
	if hostname == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "hostname 参数不能为空",
		})
		return
	}

	var records []u.Warning
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
