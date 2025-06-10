package monitor

import (
	"backend/server/logs"
	"backend/server/redis"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func GetLatestSystemInfo(c *gin.Context) {
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
	}
	cpuThreshold, err := redis.Rdb.Get(ctx, cpuKey).Float64()
	if err != nil {
		log.Printf("%s获取 CPU 阈值失败: %s", logs.GetLogPrefix(2), err)
	}
	AlertMessages := ""
	// 比较阈值并设置告警信息
	if requestData.MemInfo.UserPercent > memThreshold {
		AlertMessages = "内存告警"
	}
	for _, data := range requestData.CPUInfo {
		if data.Percent > cpuThreshold {
			AlertMessages = "CPU告警"
			if requestData.MemInfo.UserPercent > memThreshold {
				AlertMessages = "CPU与内存告警"
			}
		}
	}

	// 返回结果
	c.JSON(http.StatusOK, gin.H{
		"data":           requestData,
		"alert_messages": AlertMessages,
	})
}
