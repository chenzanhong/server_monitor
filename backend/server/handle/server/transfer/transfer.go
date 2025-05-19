package transfer

import (
	"fmt"
	"io"
	"log"
	"path"
	"strconv"
	"strings"
	"time"

	// "backend/server/handle/server/transfer/global"
	g "backend/server/handle/server/transfer/global"
	trans "backend/server/handle/server/transfer/trans-init" // 请替换为您的实际项目路径

	// m_init "backend/server/model/init"
	// u "backend/server/model/user"

	"github.com/gin-gonic/gin"
)

type RequestP2P struct {
	SourceServer string `json:"source_server"`
	TargetServer string `json:"target_server"`
	SourcePath   string `json:"source_path"`
	TargetPath   string `json:"target_path"`
	SourceUser   string `json:"source_user"`
	TargetUser   string `json:"target_user"`
	SourceAuth   string `json:"source_auth"`
	TargetAuth   string `json:"target_auth"`
}

type CommonTransRequest struct {
	Server string `json:"server" form:"server"` // 服务器地址
	Path   string `json:"path" form:"path"`     // 文件路径
	User   string `json:"user" form:"user"`     // SSH用户名
	Auth   string `json:"auth" form:"auth"`     // SSH密码或密钥
}

// 查询服务器是否是用户(所在公司)
func CheckServerBelongs(username, server string) (bool, error) {
	// 查询用户
	// var user u.User
	// if err := m_init.DB.Where("name = ?", username).First(&user).Error; err != nil {
	// 	log.Fatalf("查询用户失败: %v", err)
	// 	return false, err
	// }

	// // 查询服务器
	// var hostInfo u.HostInfo
	// if err := m_init.DB.Where("host_name = ?", server).First(&hostInfo).Error; err != nil {
	// 	log.Fatalf("查询服务器失败: %v", err)
	// 	return false, err
	// }
	// // 判断服务器是否属于用户所在的公司或者属于用户自己
	// if user.ID == hostInfo.CompanyID || user.Name == hostInfo.UserName {
	// 	return true, nil // 服务器属于用户所在的公司
	// } else {
	// 	return false, nil // 服务器不属于用户所在的公司
	// }

	return true, nil // 测试环境,暂时不做判断
}

// 指定两个服务器之间进行单文件传输
func TransferBetweenTwoServers(c *gin.Context) {
	Username, exists := c.Get("username") // 从上下文中获取用户名
	if !exists {
		c.JSON(401, gin.H{"message": "未登录"})
		return
	}

	var request RequestP2P
	if err := c.BindJSON(&request); err != nil {
		c.JSON(400, gin.H{"message": fmt.Sprintf("解析请求失败: %v", err)})
		return
	}

	flag, err := CheckServerBelongs(Username.(string), request.SourceServer)
	if err != nil {
		c.JSON(500, gin.H{"message": fmt.Sprint("查询源服务器是否属于用户（所在公司）失败: %v", err.Error())})
		return
	}
	if !flag {
		c.JSON(400, gin.H{"message": "该源服务器不属于用户（所在公司）"})
		return
	}
	flag, err = CheckServerBelongs(Username.(string), request.TargetServer)
	if err != nil {
		c.JSON(500, gin.H{"message": fmt.Sprint("查询目标服务器是否属于用户（所在公司）失败: %v", err.Error())})
		return
	}
	if !flag {
		c.JSON(400, gin.H{"message": "该目标服务器不属于用户（所在公司）"})
		return
	}

	// 判断是否存在连接池，如果不存在则创建
	if g.Pool == nil {
		g.Pool = trans.NewSSHConnectionPool(10, 5*time.Minute) // 假设容量为10，超时时间为5分钟
	}
	// 检查是否已存在到源服务器的SSH连接
	if g.FTS.Pool.Connections[request.SourceServer] == nil {
		// 如果不存在，则创建并添加到池中
		err = trans.CreateConnectionToPool(g.Pool, request.SourceServer, request.SourceUser, request.SourceAuth)
		if err != nil {
			log.Printf("创建与源服务器的连接失败: %v", err)
			c.JSON(400, gin.H{"message": fmt.Sprint("创建与源服务器的连接失败: %v", err)})
			return
		}
	}
	// 检查是否已存在到目标服务器的SSH连接
	if g.FTS.Pool.Connections[request.TargetServer] == nil {
		// 如果不存在，则创建并添加到池中
		err = trans.CreateConnectionToPool(g.Pool, request.TargetServer, request.TargetUser, request.TargetAuth)
		if err != nil {
			log.Printf("创建与目标服务器的连接失败: %v", err)
			c.JSON(400, gin.H{"message": fmt.Sprint("创建与目标服务器的连接失败: %v", err)})
			return
		}
	}

	// 执行文件传输任务
	taskID, err := g.FTS.CreateTransferBetween2STask(
		request.SourceServer, // 源服务器IP
		request.SourcePath,   // 源文件路径
		request.TargetServer, // 目标服务器IP
		request.TargetPath,   // 目标文件路径
	)
	if err != nil {
		log.Println("文件传输失败: %v", err)
	}

	fmt.Printf("文件传输任务已启动，任务ID: %s\n", taskID)
}

// 客户端与一个指定的服务器进行文件传输，上传
func CommonUpload(c *gin.Context) {
	Username, exists := c.Get("username") // 从上下文中获取用户名
	if !exists {
		c.JSON(401, gin.H{"message": "未登录"})
		return
	}

	var request CommonTransRequest
	if err := c.ShouldBind(&request); err != nil {
		c.JSON(400, gin.H{"message": fmt.Sprintf("解析请求失败: %v", err)})
		return
	}

	// 检查服务器是否属于用户所在的公司或是否是用户自己的服务器
	flag, err := CheckServerBelongs(Username.(string), request.Server)
	if err != nil {
		c.JSON(500, gin.H{"message": fmt.Sprint("查询服务器是否属于用户（所在公司）失败: %v", err)})
		return
	}
	if !flag {
		c.JSON(400, gin.H{"message": "该服务器不属于用户（所在公司）"})
		return
	}

	// 获取上传的文件
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"message": fmt.Sprintf("获取要上传的文件失败: %v", err)})
		return
	}

	// 检查是否已存在到指定服务器的SSH连接
	if g.FTS.Pool.Connections[request.Server] == nil {
		// 如果不存在，则创建并添加到池中
		err = trans.CreateConnectionToPool(g.Pool, request.Server, request.User, request.Auth)
		if err != nil {
			log.Printf("创建与目标服务器的连接失败: %v", err)
			c.JSON(400, gin.H{"message": fmt.Sprint("创建与目标服务器的连接失败: %v", err)})
			return
		}
	}

	// 执行文件传输任务
	_, err = g.FTS.CreateCommonUploadTask(
		file,
		request.Server, // 目标服务器IP
		request.Path,   // 目标文件路径
	)
	if err != nil {
		log.Printf("文件上传失败: %v", err)
		c.JSON(400, gin.H{"message": fmt.Sprintf("文件上传失败: %v", err)})
		return
	}

	c.JSON(200, gin.H{"message": "文件上传完成"})
}

// 客户端与一个指定的服务器进行文件传输，下载
func CommonDownload(c *gin.Context) {
	Username, exists := c.Get("username") // 从上下文中获取用户名
	if !exists {
		c.JSON(401, gin.H{"message": "未登录"})
		return
	}
	var request CommonTransRequest
	if err := c.BindJSON(&request); err != nil {
		c.JSON(400, gin.H{"message": fmt.Sprintf("解析请求失败: %v", err)})
		return
	}

	flag, err := CheckServerBelongs(Username.(string), request.Server)
	if err != nil {
		c.JSON(500, gin.H{"message": fmt.Sprint("查询服务器是否属于用户（所在公司）失败: %v", err)})
		return
	}
	if !flag {
		c.JSON(400, gin.H{"message": "该服务器不属于用户（所在公司）"})
		return
	}
	// 检查是否已存在到指定服务器的SSH连接
	if g.FTS.Pool.Connections[request.Server] == nil {
		// 如果不存在，则创建并添加到池中
		err = trans.CreateConnectionToPool(g.Pool, request.Server, request.User, request.Auth)
		if err != nil {
			log.Printf("创建与目标服务器的连接失败: %v", err)
			c.JSON(400, gin.H{"message": fmt.Sprint("创建与目标服务器的连接失败: %v", err)})
			return
		}
	}
	// 执行文件传输任务
	sftpClient, _, err := g.FTS.CreateCommonDownloadTask(
		request.Server,
		request.Path,
	)
	if err != nil {
		// 获取连接失败
		log.Printf("获取连接失败: %v", err)
		c.JSON(400, gin.H{"message": fmt.Sprintf("获取连接失败: %v", err)})
		return
	}
	defer sftpClient.Close()

	file, err := sftpClient.Open(request.Path) // 打开远程文件
	if err != nil {
		log.Printf("远程文件打开失败: %v", err)
		c.JSON(400, gin.H{"message": fmt.Sprintf("远程文件打开失败: %v", err)})
		return
	}
	defer file.Close()
	// 判断文件是否存在或是目录
	stat, err := file.Stat() // 获取文件信息，包括大小等
	if err != nil {
		log.Printf("文件不存在: %v", err)
		c.JSON(400, gin.H{"message": fmt.Sprintf("文件不存在: %v", err)})
		return
	}
	if stat.IsDir() {
		log.Printf("路径是一个目录: %v", err)
		c.JSON(400, gin.H{"message": fmt.Sprintf("路径是一个目录: %v", err)})
		return
	}

	filename := path.Base(request.Path)
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	fi, err := file.Stat()
	if err != nil {
		log.Printf("获取文件信息失败: %v", err)
		c.JSON(500, gin.H{"message": "获取文件信息失败"})
		return
	}
	c.Header("Content-Length", strconv.FormatInt(fi.Size(), 10))

	c.Writer.WriteHeader(200)

	if _, err := io.Copy(c.Writer, file); err != nil {
		if strings.Contains(err.Error(), "broken pipe") || err.Error() == "connection lost" {
			log.Printf("客户端已断开连接")
			return
		}
		log.Printf("文件写入响应失败: %v", err)
		return
	}
	c.Writer.Flush()
}
