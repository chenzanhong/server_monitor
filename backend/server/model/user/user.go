package model

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
	ID            	int    `json:"id" gorm:"primarykey;autoIncrement"`
	Send      		string `json:"send" gorm:"column:send"`
	Receive 		string `json:"receive" gorm:"column:receive"`
	Content       	string `json:"content" gorm:"column:content"`
	State       	string `json:"state" gorm:"column:state"`
	CreateAt    	string `json:"create_at" gorm:"column:created_at"`
}

// host_info表
type HostInfo struct {
	ID         int    `json:"id" gorm:"primarykey;autoIncrement"`
	UserName   string `json:"user_name" gorm:"column:user_name"` // 用户名
	HostName   string `json:"host_name" gorm:"column:host_name"` // 主机名
	CompanyID  int    `json:"company_id" gorm:"column:company_id"` // 公司ID
	OS         string `json:"os" gorm:"column:os"` // 操作系统
	Platform   string `json:"platform" gorm:"column:platform"` // 平台
	KernelArch string `json:"kernel_arch" gorm:"column:kernel_arch"` // 内核架构
	CreatedAt  string `json:"created_at" gorm:"column:created_at"` // 创建时间
}

// TableName 指定表名
func (HostInfo) TableName() string {
	return "host_info" // 数据库表名对应
}