package info

import (
	"fmt"
	logs "backend/server/logs"
	model "backend/server/model"
	m_init "backend/server/model/init"
	m_user "backend/server/model/user"

	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func ManageNotice(c *gin.Context) {
	Username, exists := c.Get("username")
	if !exists {
		log.Printf("用户还未登录")
		c.JSON(401, gin.H{
			"message": "用户未登录",
		})
		return
	}
	username := Username.(string)

	var requestBody m_user.Notice
	err := c.BindJSON(&requestBody)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "请求数据格式错误"})
		return
	}

	// 根据通知id查找对应的通知
	var notice m_user.Notice
	err = m_init.DB.Where("id = ?", requestBody.ID).First(&notice).Error
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "通知不存在"})
		return
	}

	if notice.State == "processed" || notice.State == "expired" {
		log.Println("通知已处理或已过期")
		c.JSON(http.StatusBadRequest, gin.H{"message": "通知已处理或已过期"})
		return
	}

	if notice.Receive != username {
		log.Println("处理的用户不是接收人")
		c.JSON(http.StatusBadRequest, gin.H{"message": "处理的用户不是接收人"})
		return
	}

	// 检查通知是否已经过期(通知的有效期限是14天)
	createAtTime, err := time.Parse(time.RFC3339, notice.CreateAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "时间格式错误"})
		return
	}

	if time.Since(createAtTime) > 14*24*time.Hour {
		update := `UPDATE notices SET state = 'expired' WHERE id = $1`
		_, err := model.DB.Exec(update, requestBody.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "更新通知状态失败"})
			log.Println("更新通知状态失败:", err)
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"message": "通知已过期"})
		return
	}

	// 查询权限
	var role_id int
	query := "SELECT role_id FROM users WHERE name = $1"
	err = model.DB.QueryRow(query, username).Scan(&role_id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "查询用户权限失败"})
		log.Println("查询用户权限失败:", err)
		return
	}

	log.Printf("开始处理通知内容: %s", notice.Content)

	// 处理申请注册公司的通知
	if strings.Contains(notice.Content, "申请注册公司") {
		// 检查是否为系统管理员
		if role_id != 2 {
			c.JSON(http.StatusBadRequest, gin.H{"message": "接收人不是系统管理员"})
			return
		}

		parts := strings.Split(notice.Content, "申请注册公司:")
		if len(parts) < 2 {
			log.Println("通知内容格式错误：缺少 '申请注册公司:' 分隔符")
			c.JSON(http.StatusBadRequest, gin.H{"message": "通知内容格式错误"})
			return
		}
		admin_username := strings.TrimSpace(parts[0])

		parts = strings.Split(parts[1], "，法人:")
		if len(parts) < 2 {
			log.Println("通知内容格式错误：缺少 '，法人:' 分隔符")
			c.JSON(http.StatusBadRequest, gin.H{"message": "通知内容格式错误"})
			return
		}
		company_name := strings.TrimSpace(parts[0])

		parts = strings.Split(parts[1], ",社会信用代码:")
		social_credit_code := strings.TrimSpace(parts[0])
		parts = strings.Split(parts[1], ",管理员邮箱:")
		// admin_email := strings.TrimSpace(parts[0])

		// 根据管理员用户名获取管理员的编号
		var admin_id int
		query = "SELECT id FROM users WHERE name = $1"
		err := model.DB.QueryRow(query, admin_username).Scan(&admin_id)
		if err != nil {
			log.Println(logs.GetLogPrefix(2) + "管理员用户名不存在")
			fmt.Println(err)
			c.JSON(http.StatusBadRequest, gin.H{"message": "管理员用户名不存在"})
			return
		}

		// 创建公司
		query = "INSERT INTO companies (admin_id, name, social_credit_code, memberNum) VALUES ($1, $2, $3, $4)"
		_, err = model.DB.Exec(query, admin_id, company_name, social_credit_code, 1)
		if err != nil {
			log.Println(logs.GetLogPrefix(2) + "创建公司失败")
			c.JSON(http.StatusInternalServerError, gin.H{"message": "创建公司失败"})
			return
		}

		// 查看公司编号
		query = "SELECT id FROM companies WHERE name = $1"
		var company_id int
		err = model.DB.QueryRow(query, company_name).Scan(&company_id)
		if err != nil {
			log.Println(logs.GetLogPrefix(2) + "查询公司编号失败")
			c.JSON(http.StatusInternalServerError, gin.H{"message": "查询公司编号失败"})
			return
		}

		// 同步更新公司管理员的company_id和role_id
		update := "UPDATE users SET company_id = $1, role_id = $2 WHERE name = $3"
		_, err = model.DB.Exec(update, company_id, 1, admin_username)
		if err != nil {
			log.Println(logs.GetLogPrefix(2) + "更新公司管理员信息失败")
			return
		}

		// 更新通知的状态为已处理
		update = "UPDATE notices SET state = $1 WHERE id = $2"
		_, err = model.DB.Exec(update, "processed", notice.ID)
		if err != nil {
			log.Println(logs.GetLogPrefix(2) + "更新通知状态失败")
			c.JSON(http.StatusInternalServerError, gin.H{"message": "更新通知状态失败"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "通知处理成功"})
		return
	}

	// 处理更换管理员的通知
	if strings.Contains(notice.Content, "申请更换公司管理") {
		// 检查是否是系统管理员
		if role_id != 2 {
			log.Println(logs.GetLogPrefix(2) + "非系统管理员无法处理通知")
			c.JSON(http.StatusUnauthorized, gin.H{"message": "非系统管理员无法处理通知"})
			return
		}

		parts := strings.Split(notice.Content, "申请更换公司管理")
		if len(parts) < 2 {
			log.Println("通知内容格式错误：缺少 '申请更换公司管理' 分隔符")
			c.JSON(http.StatusBadRequest, gin.H{"message": "通知内容格式错误"})
			return
		}
		old_admin := strings.TrimSpace(parts[0])
		log.Println(old_admin)

		parts = strings.Split(parts[1], ",新管理员用户名:")
		if len(parts) < 2 {
			log.Println("通知内容格式错误：缺少 ',新管理员用户名:' 分隔符")
			c.JSON(http.StatusBadRequest, gin.H{"message": "通知内容格式错误"})
			return
		}
		parts = strings.Split(parts[1], ",新管理员邮箱:")
		if len(parts) < 2 { 
			log.Println("通知内容格式错误：缺少 ',新管理员邮箱:' 分隔符")
			c.JSON(http.StatusBadRequest, gin.H{"message": "通知内容格式错误"})
		}
		new_admin := strings.TrimSpace(parts[0])
		log.Println(new_admin)

		// 获取原管理员公司编号
		var company_id int
		query = "select company_id from users where name = $1"
		if err := m_init.DB.Raw(query, old_admin).Scan(&company_id).Error; err != nil {
			log.Println("获取原管理员公司编号失败")
			c.JSON(http.StatusInternalServerError, gin.H{"message": "获取原管理员公司编号失败"})
			return
		}

		// 获取新公司管理员编号
		var new_admin_id int
		query = "select id from users where name = $1"
		if err := m_init.DB.Raw(query, new_admin).Scan(&new_admin_id).Error; err != nil {
			log.Println("获取新管理员编号失败")
			c.JSON(http.StatusInternalServerError, gin.H{"message": "获取新管理员编号失败"})
			return
		}

		// 更换老公司管理员权限变为0
		update := "update users set role_id = 0 where name = $1"
		if err := m_init.DB.Exec(update, old_admin).Error; err != nil {
			log.Println("更新旧管理员权限失败")
			c.JSON(http.StatusInternalServerError, gin.H{"message": "更新旧管理员权限失败"})
			return
		}
		// 更换新公司管理员权限变为1
		update = "update users set role_id = 1 where name = $1"
		if err := m_init.DB.Exec(update, new_admin).Error; err != nil {
			log.Println("更新新管理员权限失败")
			c.JSON(http.StatusInternalServerError, gin.H{"message": "更新新管理员权限失败"})
			return
		}
		// 更换公司管理员的id为新管理员id
		update = "update companies set admin_id = $1 where id = $2"
		if err := m_init.DB.Exec(update, new_admin_id, company_id).Error; err != nil {
			log.Println("更新公司管理员id失败")
			c.JSON(http.StatusInternalServerError, gin.H{"message": "更新公司管理员id失败"})
			return
		}

		// 更新通知的状态为已处理
		update = "update notices set state = $1 where id = $2"
		if err := m_init.DB.Exec(update, "processed", notice.ID).Error; err != nil {
			log.Println("更新通知状态失败")
			c.JSON(http.StatusInternalServerError, gin.H{"message": "更新通知状态失败"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "通知处理成功"})
		return
	}

	// 处理邀请加入公司的通知
	if strings.Contains(notice.Content, "邀请") && strings.Contains(notice.Content, "加入") {
		parts := strings.Split(notice.Content, "加入")
		if len(parts) < 2 || parts[1] == "" {
			log.Println("通知内容格式错误：缺少 '加入' 后的内容")
			c.JSON(http.StatusBadRequest, gin.H{"message": "通知内容格式错误"})
			return
		}
		companyName := strings.TrimSpace(parts[1])

		// 查找该公司的id
		var companyId int
		query := "SELECT id FROM companies WHERE name = $1"
		err := m_init.DB.Raw(query, companyName).Scan(&companyId).Error
		if err != nil {
			log.Println("数据库查询公司失败")
			c.JSON(http.StatusInternalServerError, gin.H{"message": "数据库查询公司失败"})
			return
		}

		// 更改新成员公司所属
		query = "UPDATE users SET company_id = $1 WHERE name = $2"
		err = m_init.DB.Exec(query, companyId, username).Error
		if err != nil {
			log.Println("数据库更新用户所属公司失败")
			c.JSON(http.StatusInternalServerError, gin.H{"message": "数据库更新用户所属公司失败"})
			return
		}
		// 更新公司成员数量
		query = "UPDATE companies SET memberNum = memberNum + 1 WHERE id = $1"
		err = m_init.DB.Exec(query, companyId).Error
		if err != nil {
			log.Println("数据库更新公司成员数量失败")
			c.JSON(http.StatusInternalServerError, gin.H{"message": "数据库更新公司成员数量失败"})
			return
		}

		// 更新通知的状态
		query = "UPDATE notices SET state = $1 WHERE id = $2"
		err = m_init.DB.Exec(query, "processed", notice.ID).Error
		if err != nil {
			log.Println("数据库更新通知状态失败")
			c.JSON(http.StatusInternalServerError, gin.H{"message": "数据库更新通知状态失败"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "通知处理成功"})
	}
}