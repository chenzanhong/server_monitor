package monitor

import (
	"log"
	"net/http"

	m_init "backend/server/model/init"

	"github.com/gin-gonic/gin"
)

type DeleteSystemInfoRequest struct {
	Host      string `json:"host"`
	User      string `json:"user"`
	Password  string `json:"password"`
	Port      int    `json:"port"`
	Host_name string `json:"host_name"`
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

	//解析json body 到结构体 DeleteSystemInfoRequest
	var deleteSystemInfoRequest DeleteSystemInfoRequest
	err := c.BindJSON(&deleteSystemInfoRequest)
	if err != nil {
		log.Println("解析json body 失败")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	//查询该主机在hostandtoken表是否存在
	query := `
		SELECT id
		FROM hostandtoken
		WHERE host_name = $1
	`
	var existingID int
	err = m_init.DB.Raw(query, deleteSystemInfoRequest.Host_name).Scan(&existingID).Error
	if err != nil {
		log.Println("数据库查询失败")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库查询失败"})
		return
	}
	if existingID > 0 {
		delete := `
			DELETE FROM hostandtoken
			WHERE host_name = $1 
		`
		if err = m_init.DB.Exec(delete, deleteSystemInfoRequest.Host_name).Error; err != nil {
			log.Println("删除hostandtoken数据失败")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "删除hostandtoken的数据失败"})
			return
		}
	}

	//检查在host_info表是否存在对应的数据
	query = `
		SELECT id
		FROM host_info
		WHERE host_name = $1 AND ip = $2 AND user_name = $3
	`
	err = m_init.DB.Raw(query, deleteSystemInfoRequest.Host_name, deleteSystemInfoRequest.Host, username).Scan(&existingID).Error
	if err != nil {
		log.Printf("查询host_info失败")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询host_info失败"})
		return
	}
	if existingID > 0 {
		delete := `
			DELETE FROM host_info
			WHERE host_name = $1 AND ip = $2 AND user_name = $3
		`
		err = m_init.DB.Exec(delete, deleteSystemInfoRequest.Host_name, deleteSystemInfoRequest.Host, username).Error
		if err != nil {
			log.Printf("删除host_info数据失败")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "删除host_info数据失败"})
			return
		}
	}

	//grpc部分
	c.JSON(http.StatusOK, gin.H{
		"message": "采集器删除成功",
	})
}

