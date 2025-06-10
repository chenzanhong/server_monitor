package getscript

import (
	cf "backend/server/config"
	pt "backend/server/handle/agent/port"
	m_init "backend/server/model/init"
	u "backend/server/model/user"
	"bytes"
	"net/http"
	"text/template"

	"github.com/gin-gonic/gin"
)

const agentTemplate = `#!/bin/bash

set -e

GITHUB_REPO="{{ .GithubRepoUrl }}"
HOSTNAME="{{ .HostName }}"
TOKEN={{ .Token }}
AGENT_DIR="$HOME/monitor"
SUDO=""

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

# 判断是否支持 systemd
if ! command -v systemctl &> /dev/null; then
  echo "[!] 当前系统不支持 systemd，无法继续安装服务"
  exit 1
fi

case "$OS" in
  ubuntu|debian)
    ${SUDO} apt update && ${SUDO} apt install -y git
    ;;
  centos|rhel)
    ${SUDO} yum install -y git
    ;;
  fedora)
    ${SUDO} dnf install -y git
    ;;
  alpine)
    su root -c "apk add --no-cache git"
    ;;
  *)
    echo "不支持的操作系统: $OS"
    exit 1
    ;;
esac

mkdir -p "$AGENT_DIR"
cd "$AGENT_DIR"

git clone "$GITHUB_REPO" .
cd agent/agent || exit

# 授予执行权限并运行主程序
chmod +x main
./main -hostname="${HOSTNAME}" -token="${TOKEN}" &

cat > /tmp/monitor_agent.service <<EOF
[Unit]
Description=Main Program Startup Service
After=network.target

[Service]
Type=simple
ExecStart=$AGENT_DIR/agent/agent/main -hostname="${HOSTNAME}" -token="${TOKEN}"
Restart=always

[Install]
WantedBy=multi-user.target
EOF

${SUDO}  mv /tmp/monitor_agent.service /etc/systemd/system/monitor_agent.service
${SUDO}  systemctl daemon-reload
${SUDO}  systemctl enable monitor_agent.service
${SUDO}  systemctl start monitor_agent.service

echo "[+] Agent 安装完成！已启动 agent 服务"
`

const sshTunnelTemplate = `#!/bin/bash

set -e

PUBLIC_SERVER_IP="{{ .PublicServerIP }}"
SSH_TUNNEL_PORT={{ .Port }}
SSH_TUNNEL_USER="{{ .SshTunnelUsername }}"

# 安装 autossh
detect_os() {
  if [ -f /etc/os-release ]; then
    . /etc/os-release
    echo "$ID"
  else
    echo "unknown"
  fi
}

OS=$(detect_os)

if command -v autossh &> /dev/null; then
  echo "[*] autossh 已安装"
else
  # 判断是否使用 sudo
  if command -v sudo &> /dev/null; then
    SUDO="sudo"
  else
    SUDO=""
  fi

  case "$OS" in
    ubuntu|debian)
      ${SUDO} apt update && ${SUDO} apt install -y autossh
      ;;
    centos|rhel)
      ${SUDO} yum install -y autossh
      ;;
    fedora)
      ${SUDO} dnf install -y autossh
      ;;
    alpine)
      su root -c "apk add --no-cache autossh"
      ;;
    *)
      echo "不支持的操作系统: $OS"
      exit 1
      ;;
  esac
fi

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

echo "[+] 反向 SSH 隧道配置完成！已启动隧道（端口: $SSH_TUNNEL_PORT）"
`

// 获取安装代理程序的脚本
func GetAgentScript(c *gin.Context) {
	hostname := c.Query("hostname")
	tmpl, err := template.New("agent").Parse(agentTemplate)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// 查询token
	var hostandtoken u.HostAndToken
	err = m_init.DB.Where("host_name = ?", hostname).First(&hostandtoken).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, "查询hostandtoken表失败："+err.Error())
		return
	}

	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", "attachment; filename=install_agent.sh")

	err = tmpl.Execute(c.Writer, struct {
		GithubRepoUrl string
		HostName      string
		Token         string
	}{
		GithubRepoUrl: cf.GithubRepoUrl,
		HostName:      hostname,
		Token:         hostandtoken.Token,
	})

	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
	}
}

func GenerateAgentScriptBytes(hostname, token string) ([]byte, error) {
	tmpl, err := template.New("agent").Parse(agentTemplate)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, struct {
		GithubRepoUrl string
		HostName      string
		Token         string
	}{
		GithubRepoUrl: cf.GithubRepoUrl,
		HostName:      hostname,
		Token:         token,
	})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// 获取配置反向SSH的脚本
func GetSSHScript(c *gin.Context) {
	port, err := pt.GetUnusedPort()
	if port == -1 {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"message": "获取反向ssh配置的脚本失败：" + err.Error()})
		return
	}

	tmpl, err := template.New("ssh").Parse(sshTunnelTemplate)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", "attachment; filename=setup_ssh_tunnel.sh")

	data := struct {
		PublicServerIP    string
		SshTunnelUsername string
		Port              int
	}{
		PublicServerIP:    cf.PublicServerIP,
		SshTunnelUsername: cf.SshTunnelUsername,
		Port:              port,
	}

	// 执行模板并将结果写入响应
	if err := tmpl.Execute(c.Writer, data); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
	}
}

// 联合脚本
const combinedScriptTemplate = `#!/bin/bash

set -e

# 第一部分：安装代理程序
GITHUB_REPO="{{ .GithubRepoUrl }}"
HOSTNAME="{{ .HostName }}"
TOKEN={{ .Token }}
AGENT_DIR="$HOME/monitor"
SUDO=""

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

# 判断是否支持 systemd
if ! command -v systemctl &> /dev/null; then
  echo "[!] 当前系统不支持 systemd，无法继续安装服务"
  exit 1
fi

case "$OS" in
  ubuntu|debian)
    ${SUDO} apt update && ${SUDO} apt install -y git
    ;;
  centos|rhel)
    ${SUDO} yum install -y git
    ;;
  fedora)
    ${SUDO} dnf install -y git
    ;;
  alpine)
    su root -c "apk add --no-cache git"
    ;;
  *)
    echo "不支持的操作系统: $OS"
    exit 1
    ;;
esac

mkdir -p "$AGENT_DIR"
cd "$AGENT_DIR"

git clone "$GITHUB_REPO" .
cd agent/agent || exit

# 授予执行权限并运行主程序
chmod +x main
./main -hostname="${HOSTNAME}" -token="${TOKEN}" &


cat > /tmp/monitor_agent.service <<EOF
[Unit]
Description=Main Program Startup Service
After=network.target

[Service]
Type=simple
ExecStart=$AGENT_DIR/main -hostname="${HOSTNAME}" -token="${TOKEN}"
Restart=always

[Install]
WantedBy=multi-user.target
EOF

sudo mv /tmp/monitor_agent.service /etc/systemd/system/monitor_agent.service
sudo systemctl daemon-reload
sudo systemctl enable monitor_agent.service
sudo systemctl start monitor_agent.service

echo "[+] Agent 安装完成！已启动 agent 服务"

# 第二部分：配置反向SSH隧道
PUBLIC_SERVER_IP="{{ .PublicServerIP }}"
SSH_TUNNEL_PORT={{ .Port }}
SSH_TUNNEL_USER="{{ .SshTunnelUsername }}"

# 安装 autossh
if command -v autossh &> /dev/null; then
  echo "[*] autossh 已安装"
else
  case "$OS" in
    ubuntu|debian)
      ${SUDO} apt update && ${SUDO} apt install -y autossh
      ;;
    centos|rhel)
      ${SUDO} yum install -y autossh
      ;;
    fedora)
      ${SUDO} dnf install -y autossh
      ;;
    alpine)
      su root -c "apk add --no-cache autossh"
      ;;
    *)
      echo "不支持的操作系统: $OS"
      exit 1
      ;;
  esac
fi

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

# 确保 systemd 目录存在
SYSTEMD_DIR="/etc/systemd/system"
if [ ! -d "$SYSTEMD_DIR" ]; then
    ${SUDO} mkdir -p "$SYSTEMD_DIR"
fi

# 移动服务文件到 systemd 目录
${SUDO} mv /tmp/reversetunnel@${SSH_TUNNEL_PORT}.service "${SYSTEMD_DIR}/reversetunnel@${SSH_TUNNEL_PORT}.service"

${SUDO} systemctl daemon-reload
${SUDO} systemctl enable reversetunnel@${SSH_TUNNEL_PORT}
${SUDO} systemctl start reversetunnel@${SSH_TUNNEL_PORT}

echo "[+] 反向 SSH 隧道配置完成！已启动隧道（端口: $SSH_TUNNEL_PORT）"
`

// 获取合并后的脚本——包含安装代理程序和配置反向SSH隧道
func GetCombinedScript(c *gin.Context) {
	hostname := c.Query("hostname")
	port, err := pt.GetUnusedPort()
	if port == -1 {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"message": "获取脚本失败：" + err.Error()})
		return
	}

	// 查询token
	var hostandtoken u.HostAndToken
	err = m_init.DB.Where("host_name = ?", hostname).First(&hostandtoken).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, "查询hostandtoken表失败："+err.Error())
		return
	}

	tmpl, err := template.New("combined").Parse(combinedScriptTemplate)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", "attachment; filename=install_agent_and_ssh_tunnel.sh")

	data := struct {
		GithubRepoUrl     string
		PublicServerIP    string
		HostName          string
		Token             string
		SshTunnelUsername string
		Port              int
	}{
		GithubRepoUrl:     cf.GithubRepoUrl,
		PublicServerIP:    cf.PublicServerIP,
		HostName:          hostname,
		Token:             hostandtoken.Token,
		SshTunnelUsername: cf.SshTunnelUsername,
		Port:              port,
	}

	if err := tmpl.Execute(c.Writer, data); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
	}
}
