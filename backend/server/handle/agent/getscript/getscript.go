package getscript

import (
	cf "backend/server/config"
	pt "backend/server/handle/agent/port"
	m_init "backend/server/model/init"
	u "backend/server/model/user"
	"bytes"
	"fmt"
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

# 检查 monitor_agent.service 是否存在或正在运行
SERVICE_EXISTS=false

if ${SUDO} systemctl is-active --quiet monitor_agent.service || ${SUDO} systemctl is-enabled --quiet monitor_agent.service || [ -f /etc/systemd/system/monitor_agent.service ]; then
    SERVICE_EXISTS=true
fi

if $SERVICE_EXISTS; then
    read -p "[!] 检测到已存在的 monitor_agent 服务，是否清理并重新安装？(y/N): " CONFIRM
    case "$CONFIRM" in
        y|Y|yes|Yes|YES)
            echo "[*] 用户选择继续清理旧服务..."
            ;;
        *)
            echo "[*] 用户取消操作，退出安装脚本。"
            exit 0
            ;;
    esac

    # 开始清理旧服务
    if ${SUDO} systemctl is-active --quiet monitor_agent.service; then
        echo "[*] 发现现有 monitor_agent 服务，正在停止..."
        ${SUDO} systemctl stop monitor_agent.service || { echo "无法停止现有服务"; exit 1; }
    fi

    if ${SUDO} systemctl is-enabled --quiet monitor_agent.service; then
        echo "[*] 禁用现有 monitor_agent 服务..."
        ${SUDO} systemctl disable monitor_agent.service || { echo "无法禁用现有服务"; exit 1; }
    fi

    if [ -f /etc/systemd/system/monitor_agent.service ]; then
        echo "[*] 正在删除旧的 monitor_agent.service 文件..."
        ${SUDO} rm /etc/systemd/system/monitor_agent.service || { echo "无法删除旧的服务文件"; exit 1; }
    fi

    ${SUDO} systemctl daemon-reload
else
    echo "[+] 准备开始安装代理程序。"
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
cd "$AGENT_DIR" || { echo "无法创建或进入目录 $AGENT_DIR"; exit 1; }

pwd

git clone "$GITHUB_REPO" .
cd agent || { echo "找不到目录 agent，请检查仓库结构"; exit 1; }

# 编译 main
# go build -o main .

# 授予执行权限并运行主程序
chmod +x main
# ./main -hostname=${HOSTNAME} -token=${TOKEN} &

cat > /tmp/monitor_agent.service <<EOF
[Unit]
Description=Main Program Startup Service
After=network.target

[Service]
Type=simple
ExecStart=$AGENT_DIR/agent/main -hostname=${HOSTNAME} -token=${TOKEN}
Restart=always

[Install]
WantedBy=multi-user.target
EOF

${SUDO}  mv /tmp/monitor_agent.service /etc/systemd/system/monitor_agent.service
${SUDO}  systemctl daemon-reload
${SUDO}  systemctl enable monitor_agent.service
${SUDO}  systemctl start monitor_agent.service
${SUDO}  systemctl status monitor_agent.service

echo "[+] Agent 安装完成！已启动 monitor_agent 服务"
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

# 定义服务名称
SERVICE_NAME="reversetunnel@${SSH_TUNNEL_PORT}"

# 检查 reversetunnel@port.service 是否存在或正在运行
SERVICE_EXISTS=false

if ${SUDO} systemctl is-active --quiet $SERVICE_NAME || ${SUDO} systemctl is-enabled --quiet $SERVICE_NAME || [ -f /etc/systemd/system/${SERVICE_NAME}.service ]; then
    SERVICE_EXISTS=true
fi

if $SERVICE_EXISTS; then
    read -p "[!] 检测到已存在的 $SERVICE_NAME 服务，是否清理并重新安装？(y/N): " CONFIRM
    case "$CONFIRM" in
        y|Y|yes|Yes|YES)
            echo "[*] 用户选择继续清理旧服务..."
            ;;
        *)
            echo "[*] 用户取消操作，保留现有服务并退出安装脚本。"
            exit 0
            ;;
    esac

    # 开始清理旧服务
    if ${SUDO} systemctl is-active --quiet $SERVICE_NAME; then
        echo "[*] 发现现有 $SERVICE_NAME 服务，正在停止..."
        ${SUDO} systemctl stop $SERVICE_NAME || { echo "无法停止现有服务"; exit 1; }
    fi

    if ${SUDO} systemctl is-enabled --quiet $SERVICE_NAME; then
        echo "[*] 禁用现有 $SERVICE_NAME 服务..."
        ${SUDO} systemctl disable $SERVICE_NAME || { echo "无法禁用现有服务"; exit 1; }
    fi

    if [ -f /etc/systemd/system/${SERVICE_NAME}.service ]; then
        echo "[*] 正在删除旧的 ${SERVICE_NAME}.service 文件..."
        ${SUDO} rm /etc/systemd/system/${SERVICE_NAME}.service || { echo "无法删除旧的服务文件"; exit 1; }
    fi

    ${SUDO} systemctl daemon-reload
else
    echo "[+] 准备开始配置反向隧道服务。"
fi

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

# 创建 systemd 服务文件
cat > /tmp/${SERVICE_NAME}.service <<EOF
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

# 移动并启用服务
${SUDO} mv /tmp/${SERVICE_NAME}.service /etc/systemd/system/
${SUDO} systemctl daemon-reload
${SUDO} systemctl enable $SERVICE_NAME
${SUDO} systemctl start $SERVICE_NAME
${SUDO} systemctl status $SERVICE_NAME

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
	hostname := c.Query("hostname")
	if hostname == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "hostname参数不能为空"})
		return
	}

	port, err := pt.GetUnusedPort()
	if port == -1 {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"message": "获取反向ssh配置的脚本失败：" + err.Error()})
		return
	}

	// 修改ssh_port表中port对应记录的hostname
	var sshport u.SSHPort
	err = m_init.DB.Where("port = ?", port).First(&sshport).Error
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "查询ssh_port表失败：" + err.Error()})
		return
	}
	sshport.Hostname = hostname
	err = m_init.DB.Save(&sshport).Error
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "更新ssh_port表失败：" + err.Error()})
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

# 检查 monitor_agent.service 是否存在或正在运行
SERVICE_EXISTS=false

if ${SUDO} systemctl is-active --quiet monitor_agent.service || ${SUDO} systemctl is-enabled --quiet monitor_agent.service || [ -f /etc/systemd/system/monitor_agent.service ]; then
    SERVICE_EXISTS=true
fi

if $SERVICE_EXISTS; then
    read -p "[!] 检测到已存在的 monitor_agent 服务，是否清理并重新安装？(y/N): " CONFIRM
    case "$CONFIRM" in
        y|Y|yes|Yes|YES)
            echo "[*] 用户选择继续清理旧服务..."
            ;;
        *)
            echo "[*] 用户取消操作，退出安装脚本。"
            exit 0
            ;;
    esac

    # 开始清理旧服务
    if ${SUDO} systemctl is-active --quiet monitor_agent.service; then
        echo "[*] 发现现有 monitor_agent 服务，正在停止..."
        ${SUDO} systemctl stop monitor_agent.service || { echo "无法停止现有服务"; exit 1; }
    fi

    if ${SUDO} systemctl is-enabled --quiet monitor_agent.service; then
        echo "[*] 禁用现有 monitor_agent 服务..."
        ${SUDO} systemctl disable monitor_agent.service || { echo "无法禁用现有服务"; exit 1; }
    fi

    if [ -f /etc/systemd/system/monitor_agent.service ]; then
        echo "[*] 正在删除旧的 monitor_agent.service 文件..."
        ${SUDO} rm /etc/systemd/system/monitor_agent.service || { echo "无法删除旧的服务文件"; exit 1; }
    fi

    ${SUDO} systemctl daemon-reload
else
    echo "[+] 准备开始安装代理程序。"
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
cd "$AGENT_DIR" || { echo "无法创建或进入目录 $AGENT_DIR"; exit 1; }

pwd

git clone "$GITHUB_REPO" .
cd agent || { echo "找不到目录 agent，请检查仓库结构"; exit 1; }

# 编译 main
# go build -o main .

# 授予执行权限并运行主程序
chmod +x main
# ./main -hostname=${HOSTNAME} -token=${TOKEN} &

cat > /tmp/monitor_agent.service <<EOF
[Unit]
Description=Main Program Startup Service
After=network.target

[Service]
Type=simple
ExecStart=$AGENT_DIR/agent/main -hostname=${HOSTNAME} -token=${TOKEN}
Restart=always

[Install]
WantedBy=multi-user.target
EOF

${SUDO}  mv /tmp/monitor_agent.service /etc/systemd/system/monitor_agent.service
${SUDO}  systemctl daemon-reload
${SUDO}  systemctl enable monitor_agent.service
${SUDO}  systemctl start monitor_agent.service
${SUDO}  systemctl status monitor_agent.service

echo "[+] Agent 安装完成！已启动 monitor_agent.service 服务"

# 第二部分：配置反向SSH隧道

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

# 定义服务名称
SERVICE_NAME="reversetunnel@${SSH_TUNNEL_PORT}"

# 检查 reversetunnel@port.service 是否存在或正在运行
SERVICE_EXISTS=false

if ${SUDO} systemctl is-active --quiet $SERVICE_NAME || ${SUDO} systemctl is-enabled --quiet $SERVICE_NAME || [ -f /etc/systemd/system/${SERVICE_NAME}.service ]; then
    SERVICE_EXISTS=true
fi

if $SERVICE_EXISTS; then
    read -p "[!] 检测到已存在的 $SERVICE_NAME 服务，是否清理并重新安装？(y/N): " CONFIRM
    case "$CONFIRM" in
        y|Y|yes|Yes|YES)
            echo "[*] 用户选择继续清理旧服务..."
            ;;
        *)
            echo "[*] 用户取消操作，保留现有服务并退出安装脚本。"
            exit 0
            ;;
    esac

    # 开始清理旧服务
    if ${SUDO} systemctl is-active --quiet $SERVICE_NAME; then
        echo "[*] 发现现有 $SERVICE_NAME 服务，正在停止..."
        ${SUDO} systemctl stop $SERVICE_NAME || { echo "无法停止现有服务"; exit 1; }
    fi

    if ${SUDO} systemctl is-enabled --quiet $SERVICE_NAME; then
        echo "[*] 禁用现有 $SERVICE_NAME 服务..."
        ${SUDO} systemctl disable $SERVICE_NAME || { echo "无法禁用现有服务"; exit 1; }
    fi

    if [ -f /etc/systemd/system/${SERVICE_NAME}.service ]; then
        echo "[*] 正在删除旧的 ${SERVICE_NAME}.service 文件..."
        ${SUDO} rm /etc/systemd/system/${SERVICE_NAME}.service || { echo "无法删除旧的服务文件"; exit 1; }
    fi

    ${SUDO} systemctl daemon-reload
else
    echo "[+] 准备开始配置反向隧道服务。"
fi

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

# 创建 systemd 服务文件
cat > /tmp/${SERVICE_NAME}.service <<EOF
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

# 移动并启用服务
${SUDO} mv /tmp/${SERVICE_NAME}.service /etc/systemd/system/
${SUDO} systemctl daemon-reload
${SUDO} systemctl enable $SERVICE_NAME
${SUDO} systemctl start $SERVICE_NAME
${SUDO} systemctl status $SERVICE_NAME

echo "[+] 反向 SSH 隧道配置完成！已启动隧道（端口: $SSH_TUNNEL_PORT）"
`

// 获取合并后的脚本——包含安装代理程序和配置反向SSH隧道
func GetCombinedScript(c *gin.Context) {
	hostname := c.Query("hostname")
	// 检查hostname参数是否为空
	if hostname == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "hostname参数不能为空"})
		return
	}

	// 查询token
	var hostandtoken u.HostAndToken
	err := m_init.DB.Where("host_name = ?", hostname).First(&hostandtoken).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, "查询hostandtoken表失败："+err.Error())
		return
	}

	port, err := pt.GetUnusedPort()
	if port == -1 {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"message": "获取脚本失败：" + err.Error()})
		return
	}

	// 修改ssh_port表中port对应记录的hostname
	var sshport u.SSHPort
	err = m_init.DB.Where("port = ?", port).First(&sshport).Error
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "查询ssh_port表失败：" + err.Error()})
		return
	}
	sshport.Hostname = hostname
	err = m_init.DB.Save(&sshport).Error
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "更新ssh_port表失败：" + err.Error()})
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

// GenerateCombinedScriptBytes 生成包含 agent 安装和 SSH 隧道配置的完整脚本字节流
func GenerateCombinedScriptBytes(hostname, token string) ([]byte, error) {
	// 检查必要配置是否存在
	if cf.PublicServerIP == "" || cf.SshTunnelUsername == "" {
		return nil, fmt.Errorf("公共服务器 IP 或隧道用户名未配置")
	}

	port, err := pt.GetUnusedPort()
	if port == -1 {
		return nil, fmt.Errorf("获取 SSH 隧道端口失败: %v", err)
	}

  // 修改ssh_port表中port对应记录的hostname
  var sshport u.SSHPort
  err = m_init.DB.Where("port = ?", port).First(&sshport).Error
  if err != nil {
    return nil, fmt.Errorf("查询ssh_port表失败：%v", err)
  }
  sshport.Hostname = hostname
  err = m_init.DB.Save(&sshport).Error
  if err != nil {
    return nil, fmt.Errorf("更新ssh_port表失败：%v", err)
  }

	// 解析模板
	tmpl, err := template.New("combined").Parse(combinedScriptTemplate)
	if err != nil {
		return nil, fmt.Errorf("解析 combined 模板失败: %v", err)
	}

	// 构造数据上下文
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
		Token:             token,
		SshTunnelUsername: cf.SshTunnelUsername,
		Port:              port,
	}

	// 渲染模板到缓冲区
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("执行 combined 模板失败: %v", err)
	}

	return buf.Bytes(), nil
}
