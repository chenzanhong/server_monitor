package monitor

import (
	"backend/server/handle/email"
	"backend/server/logs"
	m_init "backend/server/model/init"
	"backend/server/model"
	"backend/server/redis"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"gorm.io/gorm"
)

type AlertTask struct {
	RequestData RequestData
}

const (
	MaxWorkers    = 10   // 根据机器 CPU 和 IO 能力调整
	TaskQueueSize = 1000 // 队列长度，防止突发流量压垮系统
)

// monitor/worker.go
var TaskQueue chan AlertTask
var initialized bool

func StartWorkerPool(maxWorkers int, queueSize int) error {
	if initialized {
		return fmt.Errorf("worker pool already started")
	}

	TaskQueue = make(chan AlertTask, queueSize)
	for i := 0; i < maxWorkers; i++ {
		go worker()
	}
	initialized = true
	return nil
}

func worker() {
	for task := range TaskQueue {
		handleAlert(task.RequestData)
	}
}

// RequestData 用于接收系统监控数据的请求体
// @Description RequestData 包含所有需要收集的系统信息
type RequestData struct {
	CPUInfo  []model.CPUInfo     `json:"cpu_info"`  // CPU 信息
	HostInfo model.HostInfo      `json:"host_info"` // 主机信息
	MemInfo  model.MemoryInfo    `json:"mem_info"`  // 内存信息
	ProInfo  []model.ProcessInfo `json:"pro_info"`  // 进程信息
	NetInfo  []model.NetworkInfo `json:"net_info"`  // 网络信息
}

// shouldAlert 判断指定 hostname 是否满足告警的冷却时间要求
func ShouldAlert(hostname string) bool {
	var latestTime time.Time
	err := m_init.DB.Raw(`
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

func handleAlert(requestData RequestData) {
	ctx := context.Background()
	hostname := requestData.HostInfo.Hostname

	// 查询用户名，这里直接查询单个字段而非整个结构体
	var username string
	err := m_init.DB.Table("host_info").Select("username").Where("host_name = ?", hostname).Scan(&username).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			log.Printf("%s 未找到主机 %s 的信息", logs.GetLogPrefix(2), hostname)
			return // 或者采取其他措施，比如记录错误日志后返回
		} else {
			log.Printf("%s 查询主机信息失败: %s", logs.GetLogPrefix(2), err)
			return // 同样可以考虑记录错误并决定是否继续执行
		}
	}

	// 从 Redis 读取阈值
	memKey := fmt.Sprintf("mem_threshold:%s", hostname)
	cpuKey := fmt.Sprintf("cpu_threshold:%s", hostname)
	memThreshold, err := redis.Rdb.Get(ctx, memKey).Float64()
	if err != nil {
		log.Printf("%s 获取内存阈值失败: %s", logs.GetLogPrefix(2), err)
		memThreshold = 0.9 // 设置为默认值
	}
	cpuThreshold, err := redis.Rdb.Get(ctx, cpuKey).Float64()
	if err != nil {
		log.Printf("%s 获取 CPU 阈值失败: %s", logs.GetLogPrefix(2), err)
		cpuThreshold = 0.9
	}

	warningType := ""
	alertMessages := ""

	// 判断类型
	cpuAlert := false
	memAlert := false
	if requestData.MemInfo.UserPercent > memThreshold {
		memAlert = true
	}
	for _, data := range requestData.CPUInfo { // 遍历每一个CPU核心
		if data.Percent > cpuThreshold {
			cpuAlert = true
			break
		}
	}
	if cpuAlert && memAlert {
		warningType = "CPU与内存"
		alertMessages = "CPU与内存告警"
	} else if cpuAlert {
		warningType = "CPU"
		alertMessages = "CPU告警"
	} else if memAlert {
		warningType = "内存"
		alertMessages = "内存告警"
	}

	// 如果有告警信息，存储到数据库并发送邮件通知
	if warningType != "" && ShouldAlert(hostname) {
		// 查询用户邮箱
		var userEmail string
		err = m_init.DB.Raw("SELECT email FROM users WHERE name = ?", username).Scan(&userEmail).Error
		if err != nil {
			log.Printf("%s 查询用户邮箱失败: %s", logs.GetLogPrefix(2), err)
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
			`, hostname, alertMessages, requestData.CPUInfo[0].Percent, requestData.MemInfo.UserPercent, time.Now().Format("2006-01-02 15:04:05"))

			err = email.SendEmail(userEmail, subject, message)
			if err != nil {
				log.Printf("%s 发送邮件通知失败: %s", logs.GetLogPrefix(2), err)
			}
		}

		// 存储告警信息到数据库
		alertContent := fmt.Sprintf("主机 %s 发生 %s，CPU使用率: %.2f%%，内存使用率: %.2f%%",
			hostname, alertMessages, requestData.CPUInfo[0].Percent, requestData.MemInfo.UserPercent)

		err = m_init.DB.Exec(`
			INSERT INTO warnings (host_name, username, warning_type, warning_title, warning_time)
			VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)`,
			hostname, username, warningType, alertContent).Error

		if err != nil {
			log.Printf("%s 存储告警信息失败: %s", logs.GetLogPrefix(2), err)
		}
	}
}

// AddSystemInfo 接收并处理系统监控数据
//
// @Summary 接收系统监控信息（CPU、内存、主机信息等）
// @Description 该API用于接收客户端发送的系统监控数据，并验证token和JWT后将数据存储到数据库中。
// @Tags Monitor
// @Accept json
// @Produce json
// @Param request body RequestData true "请求体包含系统监控数据"
// @Success 201 {object} map[string]string "成功响应"
// @Failure 400 {object} map[string]string "无效的JSON数据或令牌长度错误"
// @Failure 401 {object} map[string]string "授权头缺失或无效的token格式或无效的JWT token"
// @Failure 500 {object} map[string]string "数据库操作失败"
// @Router /monitor [post]
func ReceiveAndStoreSystemMetrics(c *gin.Context) {
	// 解析请求数据
	var requestData RequestData
	if err := c.ShouldBindJSON(&requestData); err != nil {
		s := fmt.Sprintf("Invalid JSON data: %s", err)
		log.Printf("%sInvalid JSON data: %s", logs.GetLogPrefix(2), err)
		c.JSON(http.StatusBadRequest, gin.H{"error": s})
		return
	}

	// 将数据插入 Redis
	ctx := context.Background()
	timestamp := time.Now().Unix() // 获取当前时间戳
	key := fmt.Sprintf("system_info:%s:%d", requestData.HostInfo.Hostname, timestamp)
	jsonData, err := json.Marshal(requestData)
	if err != nil {
		log.Printf("%s Failed to marshal data to JSON: %s", logs.GetLogPrefix(2), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal data to JSON"})
		return
	}

	// 将 JSON 字符串存储到 Redis
	err = redis.Rdb.Set(ctx, key, jsonData, 30*time.Minute).Err()
	if err != nil {
		log.Printf("%sFailed to insert data into Redis: %s", logs.GetLogPrefix(2), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to insert data into Redis"})
		return
	}

	// 入队而不是直接开 goroutine
	select {
	case TaskQueue <- AlertTask{RequestData: requestData}:
		// 成功入队
	default:
		// 任务队列已满，启动单独的协程进行处理
		go handleAlert(requestData)
	}

	c.JSON(http.StatusCreated, gin.H{"message": "System information inserted successfully"})
}
