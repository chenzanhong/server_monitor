package monitor

import (
	"backend/server/handle/email"
	"backend/server/logs"
	model "backend/server/model/init"
	"backend/server/redis"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// shouldAlert 判断指定 hostname 是否满足告警的冷却时间要求
func ShouldAlert(hostname string) bool {
	var latestTime time.Time
	err := model.DB.Raw(`
		SELECT warning_time FROM warnings 
		WHERE host_name = ? 
		ORDER BY warning_time DESC LIMIT 1`,
		hostname).Scan(&latestTime).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// 没有历史记录，可以告警
			return true
		} else { // 数据库查询失败
			return false
		}
	}

	// 获取当前时间
	now := time.Now()

	// 冷静期为10分钟
	coolDownPeriod := 10 * time.Minute

	// 返回是否已经超过冷静期
	return now.Sub(latestTime) > coolDownPeriod
}

func GetLatestSystemInfo(c *gin.Context) {
	Username, exists := c.Get("username")
	if !exists {
		log.Printf("未找到用户信息")
		c.JSON(401, gin.H{
			"message": "未找到用户信息",
		})
		return
	}
	username := Username.(string)
	hostname := c.Param("hostname")
	if len(hostname) == 0 {
		log.Printf("%s名字出错！", logs.GetLogPrefix(2))
		c.JSON(http.StatusBadRequest, gin.H{"error": "主机名不能为空"})
		return
	}

	ctx := context.Background()
	var latestKey string
	var latestTimestamp int64

	// 使用 SCAN 命令查找最新的键
	var cursor uint64
	for {
		keys, nextCursor, err := redis.Rdb.Scan(ctx, cursor, fmt.Sprintf("system_info:%s:*", hostname), 100).Result()
		if err != nil {
			log.Printf("%sError scanning keys: %v\n", logs.GetLogPrefix(2), err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan keys"})
			return
		}

		// 遍历键，找到最新的时间戳
		for _, key := range keys {
			timestampStr := key[len(fmt.Sprintf("system_info:%s:", hostname)):]
			timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
			if err != nil {
				log.Printf("%sError parsing timestamp from key %s: %v\n", logs.GetLogPrefix(2), key, err)
				continue
			}
			if timestamp > latestTimestamp {
				latestTimestamp = timestamp
				latestKey = key
			}
		}

		// 如果遍历完成，退出循环
		if nextCursor == 0 {
			break
		}
		cursor = nextCursor
	}

	// 获取最新数据
	if latestKey == "" {
		log.Printf("%sNo data found in Redis", logs.GetLogPrefix(2))
		c.JSON(http.StatusNotFound, gin.H{"error": "No data found in Redis"})
		return
	}

	// 从 Redis 获取 JSON 字符串
	jsonData, err := redis.Rdb.Get(ctx, latestKey).Result()
	if err != nil {
		log.Printf("%s获取 Redis 数据失败: %s", logs.GetLogPrefix(2), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取数据失败"})
		return
	}

	// 反序列化 JSON 字符串为 RequestData 结构体
	var requestData RequestData
	err = json.Unmarshal([]byte(jsonData), &requestData)
	if err != nil {
		log.Printf("%s解析 JSON 数据失败: %s", logs.GetLogPrefix(2), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解析数据失败"})
		return
	}

	// 从 Redis 读取阈值
	memKey := fmt.Sprintf("mem_threshold:%s", hostname)
	cpuKey := fmt.Sprintf("cpu_threshold:%s", hostname)
	memThreshold, err := redis.Rdb.Get(ctx, memKey).Float64()
	if err != nil {
		log.Printf("%s获取内存阈值失败: %s", logs.GetLogPrefix(2), err)
		memThreshold = 0.9 // 设置为默认值
	}
	cpuThreshold, err := redis.Rdb.Get(ctx, cpuKey).Float64()
	if err != nil {
		log.Printf("%s获取 CPU 阈值失败: %s", logs.GetLogPrefix(2), err)
		cpuThreshold = 0.9
	}

	warningType := ""
	AlertMessages := ""

	// 判断类型
	cpuAlert := false
	memAlert := false
	if requestData.MemInfo.UserPercent > memThreshold {
		memAlert = true
	}
	for _, data := range requestData.CPUInfo {
		if data.Percent > cpuThreshold {
			cpuAlert = true
			break
		}
	}
	if cpuAlert && memAlert {
		warningType = "CPU与内存"
		AlertMessages = "CPU与内存告警"
	} else if cpuAlert {
		warningType = "CPU"
		AlertMessages = "CPU告警"
	} else if memAlert {
		warningType = "内存"
		AlertMessages = "内存告警"
	}

	// 如果有告警信息，存储到数据库并发送邮件通知
	if warningType != "" && ShouldAlert(hostname) {
		// 查询用户邮箱
		var userEmail string
		err = model.DB.Raw("SELECT email FROM users WHERE name = ?", username).Scan(&userEmail).Error
		if err != nil {
			log.Printf("%s查询用户邮箱失败: %s", logs.GetLogPrefix(2), err)
		} else if userEmail != "" {
			// 发送邮件通知
			subject := fmt.Sprintf("系统告警通知 - %s", hostname)
			message := fmt.Sprintf(`
				<h2>系统告警通知</h2>
				<p>主机名: %s</p>
				<p>告警类型: %s</p>
				<p>CPU使用率: %.2f%%</p>
				<p>内存使用率: %.2f%%</p>
				<p>时间: %s</p>
			`, hostname, AlertMessages, requestData.CPUInfo[0].Percent, requestData.MemInfo.UserPercent, time.Now().Format("2006-01-02 15:04:05"))

			err = email.SendEmail(userEmail, subject, message)
			if err != nil {
				log.Printf("%s发送邮件通知失败: %s", logs.GetLogPrefix(2), err)
			}
		}

		// 存储告警信息到数据库
		alertContent := fmt.Sprintf("主机 %s 发生 %s，CPU使用率: %.2f%%，内存使用率: %.2f%%",
			hostname, AlertMessages, requestData.CPUInfo[0].Percent, requestData.MemInfo.UserPercent)

		err = model.DB.Exec(`
			INSERT INTO warnings (host_name, warning_type, warning_title, warning_time)
			VALUES (?, ?, ?, CURRENT_TIMESTAMP)`,
			hostname, warningType, alertContent).Error

		if err != nil {
			log.Printf("%s存储告警信息失败: %s", logs.GetLogPrefix(2), err)
		}
	}

	// 返回结果
	c.JSON(http.StatusOK, gin.H{
		"data":           requestData,
		"alert_messages": AlertMessages,
	})
}
