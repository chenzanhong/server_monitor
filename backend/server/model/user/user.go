package model

import "time"

// import "time"

type User struct {
	ID         int    `json:"id" gorm:"primarykey;autoIncrement"`
	Name       string `json:"name" gorm:"column:name; not null"`
	Realname   string `json:"realname" gorm:"column:realname; not null"`
	Email      string `json:"email" gorm:"unique;not null"`
	Password   string `json:"password" gorm:"not null"`
	RoleId     int    `json:"role_id" gorm:"column:role_id;default:0"` // 2:ROOT: , 1: ADMIN, 0: USER
	CompanyId  int    `json:"company_id" gorm:"column:company_id;default:0"`
	IsVerified bool   `json:"is_verified" gorm:"column:is_verified"`
	Token      string `json:"token" gorm:"column:token"`
}

type Company struct {
	ID               int    `json:"id" gorm:"primarykey;autoIncrement"`
	Name             string `json:"name" gorm:"column:name; not null"`
	SocialCreditCode string `json:"social_credit_code" gorm:"column:social_credit_code; not null"`
	Description      string `json:"description" gorm:"column:description; not null"`
	AdminID          int    `json:"admin_id" gorm:"column:admin_id; not null"`
	MemberNum        int    `json:"membernum" gorm:"column:membernum; default:0"`
	SystemNum        int    `json:"systemnum" gorm:"column:systemnum; default:0"`
}

type Role struct {
	ID          int    `json:"id" gorm:"column:id"`
	Name        string `json:"name" gorm:"column:role_name"`
	Description string `json:"description" gorm:"description"`
}

type SSHKey struct {
	ID       int    `json:"id" gorm:"primarykey;autoIncrement"`
	Hostname string `json:"host_name" gorm:"column:host_name"`
	SSHKey   string `json:"sshkey" gorm:"column:sshkey"`
}

type Notice struct {
	ID       int    `json:"id" gorm:"primarykey;autoIncrement"`
	Send     string `json:"send" gorm:"column:send"`
	Receive  string `json:"receive" gorm:"column:receive"`
	Content  string `json:"content" gorm:"column:content"`
	State    string `json:"state" gorm:"column:state"`
	CreateAt string `json:"created_at" gorm:"column:created_at"`
}

// host_info表
type HostInfo struct {
	ID         int    `json:"id"`        // 添加 ID 字段
	UserName   string `json:"user_name"` // 新增字段对应 user_name
	Hostname   string `json:"host_name"` // 原名 host_name
	IP         string `json:"ip"`
	OS         string `json:"os"`
	Platform   string `json:"platform"`
	KernelArch string `json:"kernel_arch"`
	CreatedAt  string `json:"host_info_created_at"` // 对应 created_at
	CompanyID  int    `json:"company_id,omitempty"` // 新增字段对应 company_id, 使用指针类型表示可选值
}

// TableName 指定表名
func (HostInfo) TableName() string {
	return "host_info" // 数据库表名对应
}

type HostAndToken struct {
	ID            int    `json:"id"`
	HostName      string `json:"host_name"`
	Token         string `json:"token"`
	LastHeartBeat string `json:"last_heartbeat"`
	Status        string `json:"status" gorm:"default 'offline'"`
}

type SSHPort struct {
	ID         int       `json:"id"`
	Port       int       `json:"port" gorm:"column:port"`
	IsUsed     bool      `json:"is_used" gorm:"default:false"`
	AssignedTo string    `json:"assigned_to" gorm:"column:assigned_to"`
	UpdatedAt  time.Time `json:"update_at" gorm:"column:update_at"`
}
