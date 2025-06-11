package monitor

import (
	"log"
	"net/http"

	m_init "backend/server/model/init"

	"github.com/gin-gonic/gin"
)

type DeleteSystemInfoRequest struct {
	IP       string `json:"ip"`
	HostName string `json:"host_name"`
}

func DeleteSystemInfo(c *gin.Context) {
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

	// 解析 json body 到结构体 DeleteSystemInfoRequest
	var deleteSystemInfoRequest DeleteSystemInfoRequest
	err := c.BindJSON(&deleteSystemInfoRequest)
	if err != nil {
		log.Println("解析 json body 失败")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 开始事务
	tx := m_init.DB.Begin()
	if tx.Error != nil {
		log.Println("开始事务失败")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "开始事务失败"})
		return
	}

	// 查询该主机在 hostandtoken 表是否存在
	query := `
        SELECT id
        FROM hostandtoken
        WHERE host_name = $1
    `
	var existingID int
	err = tx.Raw(query, deleteSystemInfoRequest.HostName).Scan(&existingID).Error
	if err != nil {
		log.Println("数据库查询失败")
		tx.Rollback() // 回滚事务
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库查询失败"})
		return
	}

	if existingID > 0 {
		deleteQuery := `
            DELETE FROM hostandtoken
            WHERE host_name = $1 
        `
		if err = tx.Exec(deleteQuery, deleteSystemInfoRequest.HostName).Error; err != nil {
			log.Println("删除 hostandtoken 数据失败")
			tx.Rollback() // 回滚事务
			c.JSON(http.StatusInternalServerError, gin.H{"error": "删除 hostandtoken 的数据失败"})
			return
		}
	} else {
		log.Println("数据库没有相应的 hostandtoken 数据")
		tx.Rollback() // 回滚事务
		c.JSON(http.StatusBadRequest, gin.H{"error": "数据库没有相应的 hostandtoken 数据"})
		return
	}

	// 检查在 host_info 表是否存在对应的数据
	query = `
        SELECT id
        FROM host_info
        WHERE host_name = $1 AND ip = $2 AND user_name = $3
    `
	err = tx.Raw(query, deleteSystemInfoRequest.HostName, deleteSystemInfoRequest.IP, username).Scan(&existingID).Error
	if err != nil {
		log.Printf("查询 host_info 失败")
		tx.Rollback() // 回滚事务
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询 host_info 失败"})
		return
	}

	if existingID > 0 {
		deleteQuery := `
            DELETE FROM host_info
            WHERE host_name = $1 AND ip = $2 AND user_name = $3
        `
		if err = tx.Exec(deleteQuery, deleteSystemInfoRequest.HostName, deleteSystemInfoRequest.IP, username).Error; err != nil {
			log.Printf("删除 host_info 数据失败")
			tx.Rollback() // 回滚事务
			c.JSON(http.StatusInternalServerError, gin.H{"error": "删除 host_info 数据失败"})
			return
		}
	} else {
		log.Println("数据库没有相应的 host_info 数据")
		tx.Rollback() // 回滚事务
		c.JSON(http.StatusBadRequest, gin.H{"error": "数据库没有相应的 host_info 数据"})
		return
	}

	// 提交事务
	if err = tx.Commit().Error; err != nil {
		log.Println("提交事务失败")
		tx.Rollback() // 回滚事务
		c.JSON(http.StatusInternalServerError, gin.H{"error": "提交事务失败"})
		return
	}

	// 成功响应
	c.JSON(http.StatusOK, gin.H{
		"message": "采集器删除成功",
	})
}
