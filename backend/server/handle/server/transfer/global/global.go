package global

import (
	"errors"
	"io"
	"log"
	"mime/multipart"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/sftp"

	"golang.org/x/crypto/ssh"
)

type SSHConnection struct {
	Client *ssh.Client
	UsedAt time.Time
}

type SSHConnectionPool struct {
	sync.Mutex
	Connections map[string]*SSHConnection // key可以是server IP或者标识符
	Capacity    int                       // 每个服务器的最大连接数
	Timeout     time.Duration             // 连接的最大空闲时间
}

// 定义一个具体类型来实现FileTransferService接口
type FileTransferServiceImpl struct {
	Pool *SSHConnectionPool
}

type FileTransferService interface {
	CreateTransferTask(srcServer, destServer, srcPath, destPath string) (string, error)
	GetTransferStatus(taskID string) (string, error)
}

var Pool *SSHConnectionPool // 全局连接池

var FTS *FileTransferServiceImpl // 全局文件传输服务

// 添加连接到连接池中
func (p *SSHConnectionPool) Add(server string, client *ssh.Client) {
	p.Lock()
	defer p.Unlock()

	// 如果已有连接，先关闭旧的
	if oldConn, exists := p.Connections[server]; exists {
		p.Put(server, oldConn.Client) // 放回旧连接
	}

	if len(p.Connections) >= p.Capacity {
		client.Close() // 超过容量则关闭连接
		return
	}

	p.Connections[server] = &SSHConnection{
		Client: client,
		UsedAt: time.Now(),
	}
}

// 从连接池中获取连接
func (p *SSHConnectionPool) Get(server string) (*ssh.Client, error) {
	p.Lock()
	defer p.Unlock()

	if conn, exists := p.Connections[server]; exists {
		if time.Since((*conn).UsedAt) > p.Timeout {
			conn.Client.Close() // 连接超时，关闭并删除
			delete(p.Connections, server)
		} else {
			(*conn).UsedAt = time.Now() // 更新使用时间
			return (*conn).Client, nil
		}
	}
	// 如果不存在有效连接，返回错误
	return nil, errors.New("no available connection")
}

// 放回/更新连接到连接池中
func (p *SSHConnectionPool) Put(server string, client *ssh.Client) {
	p.Lock()
	defer p.Unlock()

	// 如果已有连接，先关闭旧的
	if oldConn, exists := p.Connections[server]; exists {
		oldConn.Client.Close() // 关闭旧连接
		// 更新连接
		delete(p.Connections, server)
		// 添加新连接
		p.Connections[server] = &SSHConnection{
			Client: client,
			UsedAt: time.Now(),
		}
	}
}

// 定期清理连接池
func (p *SSHConnectionPool) Cleanup(stopChan chan struct{}) {
	ticker := time.NewTicker(p.Timeout / 2) // 每半超时时间检查一次
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.Lock()
			now := time.Now()
			for server, conn := range p.Connections {
				if now.Sub(conn.UsedAt) > p.Timeout {
					conn.Client.Close() // 连接超时，关闭并删除
					delete(p.Connections, server)
				}
			}
			p.Unlock()
		case <-stopChan:
			return // 停止清理
		}
	}
}

// 创建普通传输任务：客户端上传文件给指定服务器
func (fts *FileTransferServiceImpl) CreateCommonUploadTask(file *multipart.FileHeader, server, path string) (string, error) {
	// 获取连接（不放回，因为传输过程中需要保持连接）
	client, err := fts.Pool.Get(server)
	if err != nil {
		return "", err
	}

	// 创建SFTP客户端
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		fts.Pool.Put(server, client) // 放回连接
		return "", err
	}
	defer sftpClient.Close() // 确保在函数结束时关闭SFTP客户端

	// 实际传输逻辑
	srcFile, err := file.Open()
	if err != nil {
		return "", err
	}
	defer srcFile.Close()

	destFile, err := sftpClient.Create(path) // 创建远程文件
	if err != nil {
		return "", err
	}
	defer destFile.Close()

	// 复制文件内容
	if _, err := io.Copy(destFile, srcFile); err != nil {
		log.Printf("文件复制失败: %v", err)
		return "", err
	}

	// 确保文件权限正确
	if err := sftpClient.Chmod(path, 0644); err != nil { // 假设目标文件需要0644权限
		log.Printf("文件权限设置失败: %v", err)
		return "", err
	}
	// 传输完成后放回连接
	fts.Pool.Put(server, client)

	// 生成任务ID
	taskID := uuid.New().String()

	return taskID, nil
}

// 创建普通传输任务：客户端下载文件给指定服务器
func (fts *FileTransferServiceImpl) CreateCommonDownloadTask(server, path string) (*sftp.Client, string, error) {
	// 获取连接（不放回，因为传输过程中需要保持连接）
	client, err := fts.Pool.Get(server)
	if err != nil {
		return nil, "", err
	}
	// 创建SFTP客户端
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		fts.Pool.Put(server, client) // 放回连接
		return nil, "", err
	}

	// 不Put，因为需要保持连接
	// fts.Pool.Put(server, client) 

	// 生成任务ID
	taskID := uuid.New().String()

	return sftpClient, taskID, nil
}

// 创建两个服务器间的传输任务
func (fts *FileTransferServiceImpl) CreateTransferBetween2STask(srcServer, srcPath, destServer, destPath string) (string, error) {
	// 获取连接（不放回，因为传输过程中需要保持连接）
	srcClient, err := fts.Pool.Get(srcServer)
	if err != nil {
		return "", err
	}

	destClient, err := fts.Pool.Get(destServer)
	if err != nil {
		fts.Pool.Put(srcServer, srcClient) // 放回源连接
		return "", err
	}

	// 创建SFTP客户端
	srcSftp, err := sftp.NewClient(srcClient)
	if err != nil {
		fts.Pool.Put(srcServer, srcClient)
		fts.Pool.Put(destServer, destClient)
		return "", err
	}

	destSftp, err := sftp.NewClient(destClient)
	if err != nil {
		srcSftp.Close()
		fts.Pool.Put(srcServer, srcClient)
		fts.Pool.Put(destServer, destClient)
		return "", err
	}

	// 实际传输逻辑
	srcFile, err := srcSftp.Open(srcPath)
	if err != nil {
		srcSftp.Close()
		destSftp.Close()
		fts.Pool.Put(srcServer, srcClient)
		fts.Pool.Put(destServer, destClient)
		return "", err
	}
	defer srcFile.Close()

	destFile, err := destSftp.Create(destPath)
	if err != nil {
		srcSftp.Close()
		destSftp.Close()
		fts.Pool.Put(srcServer, srcClient)
		fts.Pool.Put(destServer, destClient)
		return "", err
	}
	defer destFile.Close()

	// 复制文件内容
	if _, err := io.Copy(destFile, srcFile); err != nil {
		log.Printf("文件复制失败: %v", err)
		srcSftp.Close()
		destSftp.Close()
		fts.Pool.Put(srcServer, srcClient)
		fts.Pool.Put(destServer, destClient)
		return "", err
	}

	// 确保文件权限正确
	if err := destSftp.Chmod(destPath, 0644); err != nil { // 假设目标文件需要0644权限
		log.Printf("文件权限设置失败: %v", err)
		srcSftp.Close()
		destSftp.Close()
		fts.Pool.Put(srcServer, srcClient)
		fts.Pool.Put(destServer, destClient)
		return "", err
	}

	// 关闭SFTP客户端
	if err := srcSftp.Close(); err != nil {
		log.Printf("源SFTP客户端关闭失败: %v", err)
	}

	// 传输完成后放回连接
	fts.Pool.Put(srcServer, srcClient)
	fts.Pool.Put(destServer, destClient)

	// 生成任务ID
	taskID := uuid.New().String()

	return taskID, nil
}

func (fts *FileTransferServiceImpl) GetTransferStatus(taskID string) (string, error) {
	// 实现获取任务状态的逻辑
	return "", nil
}
