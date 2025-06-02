package install

import (
	"net/http"
	"text/template"

	"github.com/gin-gonic/gin"
)

const scriptTemplate = `#!/bin/bash

set -e

PUBLIC_SERVER_IP="47.86.232.20"
SSH_TUNNEL_PORT={{ .Port }}
SSH_TUNNEL_USER="reversessh"
GITHUB_REPO="https://gitee.com/wu-jinhao111/agent.git"
AGENT_DIR="$HOME/agent/agent"

# 安装依赖
detect_os() {
  if [ -f /etc/os-release ]; then
    . /etc/os-release
    echo "$ID"
  else
    echo "unknown"
  fi
}

OS=$(detect_os)

# 判断是否使用 sudo
if command -v sudo &> /dev/null; then
  SUDO="sudo"
else
  SUDO=""
fi

# ========== 安装依赖 ==========
detect_os() {
  if [ -f /etc/os-release ]; then
    . /etc/os-release
    echo "$ID"
  else
    echo "unknown"
  fi
}

OS=$(detect_os)

# 判断是否使用 sudo
if command -v sudo &> /dev/null; then
  SUDO="sudo"
else
  SUDO=""
fi

# 判断是否支持 systemd
if ! command -v systemctl &> /dev/null; then
  echo "[!] 当前系统不支持 systemd，无法继续安装服务"
  exit 1
fi

case "$OS" in
  ubuntu|debian)
    ${SUDO} apt update && ${SUDO} apt install -y git autossh ssh
    ;;
  centos|rhel)
    ${SUDO} yum install -y git autossh openssh-clients
    ;;
  fedora)
    ${SUDO} dnf install -y git autossh openssh-clients
    ;;
  alpine)
    su root -c "apk add --no-cache git autossh openssh-client"
    ;;
  *)
    echo "不支持的操作系统: $OS"
    exit 1
    ;;
esac

# 创建目录并运行 agent
mkdir -p "$AGENT_DIR"
cd "$AGENT_DIR"

if [ ! -f "main" ]; then
  git clone "$GITHUB_REPO" .
fi

chmod +x main
./main -hostname="$(hostname)" &

# 创建 systemd 服务
cat > /tmp/main_startup.service <<EOF
[Unit]
Description=Main Program Startup Service
After=network.target

[Service]
Type=simple
ExecStart=$AGENT_DIR/main -hostname=$(hostname)
Restart=always

[Install]
WantedBy=multi-user.target
EOF

sudo mv /tmp/main_startup.service /etc/systemd/system/main_startup.service
sudo systemctl daemon-reload
sudo systemctl enable main_startup.service
sudo systemctl start main_startup.service

# 配置反向隧道
cat > /tmp/reversetunnel@${SSH_TUNNEL_PORT}.service <<EOF
[Unit]
Description=Reverse SSH Tunnel on Port %i
After=network.target

[Service]
User=$(whoami)
ExecStart=/usr/bin/autossh -M 0 -N -o "StrictHostKeyChecking=no" -R %i:localhost:22 ${SSH_TUNNEL_USER}@${PUBLIC_SERVER_IP}
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

sudo mv /tmp/reversetunnel@${SSH_TUNNEL_PORT}.service /etc/systemd/system/reversetunnel@${SSH_TUNNEL_PORT}.service
sudo systemctl daemon-reload
sudo systemctl enable reversetunnel@${SSH_TUNNEL_PORT}
sudo systemctl start reversetunnel@${SSH_TUNNEL_PORT}

echo "[+] 安装完成！已启动 agent 和反向隧道（端口: $SSH_TUNNEL_PORT）"
`

type ScriptRequest struct {
	OS string `json:"os" form:"os"`
}

var portPool = []int{2222, 2223, 2224, 2225, 2226} // 可用端口池
var usedPorts = make(map[int]bool)

func getNextAvailablePort() int {
	for _, port := range portPool {
		if !usedPorts[port] {
			usedPorts[port] = true
			return port
		}
	}
	return -1 // 没有空闲端口
}

// GenerateScript 动态生成安装脚本
func GenerateScript(c *gin.Context) {
	var req ScriptRequest
	if err := c.Bind(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	port := getNextAvailablePort()
	if port == -1 {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "没有可用端口"})
		return
	}

	tmpl, _ := template.New("script").Parse(scriptTemplate)
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", "attachment; filename=install.sh")

	tmpl.Execute(c.Writer, struct {
		Port int
	}{
		Port: port,
	})
}
