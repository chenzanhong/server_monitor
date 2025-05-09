package trans

import (
	g "backend/server/handle/server/transfer/global"
	"fmt"
	"log"
	"time"

	"golang.org/x/crypto/ssh"
)

// 提供一个创建服务实例的方法
func NewFileTransferService(pool *g.SSHConnectionPool) g.FileTransferServiceImpl {
	return g.FileTransferServiceImpl{Pool: pool}
}

// 提供一个创建连接池的方法
func NewSSHConnectionPool(capacity int, timeout time.Duration) *g.SSHConnectionPool {
	return &g.SSHConnectionPool{
		Connections: make(map[string]*g.SSHConnection),
		Capacity:    capacity,
		Timeout:     timeout,
	}
}

// CreateConnectionToPool 创建一个SSH连接并添加到连接池中
func CreateConnectionToPool(pool *g.SSHConnectionPool,server, user, auth string) {
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			// 假设auth是私钥的内容，如果是密码，请调整此处
			ssh.Password(auth), // 如果是使用私钥，则应使用ssh.PrivateKey
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // 在生产环境中应该使用更安全的方式
	}

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", server), config)
	if err != nil {
		log.Fatalf("无法连接到服务器 %s: %v", server, err)
	}

	pool.Add(server, client)
}