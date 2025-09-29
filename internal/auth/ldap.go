package auth

import (
	"fmt"
	"log"

	"github.com/go-ldap/ldap/v3"
)

type LDAPConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	BaseDN   string `yaml:"base_dn"`
	UserDN   string `yaml:"user_dn"`
	BindDN   string `yaml:"bind_dn"`
	BindPass string `yaml:"bind_pass"`
}

type LDAPUser struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	FullName string `json:"full_name"`
}

var ldapConfig *LDAPConfig

// InitLDAP 初始化LDAP配置
func InitLDAP(config *LDAPConfig) {
	ldapConfig = config
	log.Printf("LDAP配置已初始化: %s:%d", config.Host, config.Port)
}

// AuthenticateUser 验证用户凭据
func AuthenticateUser(username, password string) (*LDAPUser, error) {
	if ldapConfig == nil {
		return nil, fmt.Errorf("LDAP未配置")
	}

	conn, err := ldap.Dial("tcp", fmt.Sprintf("%s:%d", ldapConfig.Host, ldapConfig.Port))
	if err != nil {
		return nil, fmt.Errorf("连接LDAP服务器失败: %w", err)
	}
	defer conn.Close()

	// 绑定管理员用户
	if ldapConfig.BindDN != "" {
		err = conn.Bind(ldapConfig.BindDN, ldapConfig.BindPass)
		if err != nil {
			return nil, fmt.Errorf("LDAP管理员绑定失败: %w", err)
		}
	}

	// 搜索用户
	searchRequest := ldap.NewSearchRequest(
		ldapConfig.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(uid=%s)", username),
		[]string{"dn", "uid", "mail", "cn", "displayName"},
		nil,
	)

	sr, err := conn.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("搜索用户失败: %w", err)
	}

	if len(sr.Entries) != 1 {
		return nil, fmt.Errorf("用户不存在或找到多个用户")
	}

	userDN := sr.Entries[0].DN

	// 验证用户密码
	err = conn.Bind(userDN, password)
	if err != nil {
		return nil, fmt.Errorf("用户名或密码错误")
	}

	// 提取用户信息
	entry := sr.Entries[0]
	user := &LDAPUser{
		Username: entry.GetAttributeValue("uid"),
		Email:    entry.GetAttributeValue("mail"),
		FullName: entry.GetAttributeValue("cn"),
	}

	if user.FullName == "" {
		user.FullName = entry.GetAttributeValue("displayName")
	}

	return user, nil
}

// TestConnection 测试LDAP连接
func TestConnection() error {
	if ldapConfig == nil {
		return fmt.Errorf("LDAP未配置")
	}

	conn, err := ldap.Dial("tcp", fmt.Sprintf("%s:%d", ldapConfig.Host, ldapConfig.Port))
	if err != nil {
		return fmt.Errorf("连接LDAP服务器失败: %w", err)
	}
	defer conn.Close()

	if ldapConfig.BindDN != "" {
		err = conn.Bind(ldapConfig.BindDN, ldapConfig.BindPass)
		if err != nil {
			return fmt.Errorf("LDAP管理员绑定失败: %w", err)
		}
	}

	return nil
}
