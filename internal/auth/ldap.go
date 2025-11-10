package auth

import (
	"errors"
	"fmt"
	"strings"

	"GPUConductor/internal/logger"
	"github.com/go-ldap/ldap/v3"
	"go.uber.org/zap"
)

type LDAPConfig struct {
	Host       string `yaml:"host"`
	Port       int    `yaml:"port"`
	BaseDN     string `yaml:"base_dn"`
	UserDN     string `yaml:"user_dn"`
	BindDN     string `yaml:"bind_dn"`
	BindPass   string `yaml:"bind_pass"`
	UserFilter string `yaml:"user_filter"`
}

type LDAPUser struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	FullName string `json:"full_name"`
	Mobile   string `json:"mobile"`
}

var ldapConfig *LDAPConfig
var ErrLDAPNotConfigured = errors.New("ldap not configured")

// InitLDAP 初始化LDAP配置
func InitLDAP(config *LDAPConfig) {
	ldapConfig = config
	logger.Info("LDAP配置已初始化",
		zap.String("host", config.Host),
		zap.Int("port", config.Port),
		zap.String("base_dn", config.BaseDN),
		zap.String("user_dn", config.UserDN),
		zap.Bool("has_bind_dn", config.BindDN != ""),
	)
}

// AuthenticateUser 验证用户凭据
func AuthenticateUser(username, password string) (*LDAPUser, error) {
	logger.Info("开始LDAP用户认证", zap.String("username", username))

	if ldapConfig == nil {
		logger.Error("LDAP配置未初始化")
		return nil, ErrLDAPNotConfigured
	}

	logger.Debug("连接LDAP服务器",
		zap.String("host", ldapConfig.Host),
		zap.Int("port", ldapConfig.Port),
	)

	conn, err := ldap.Dial("tcp", fmt.Sprintf("%s:%d", ldapConfig.Host, ldapConfig.Port))
	if err != nil {
		logger.Error("连接LDAP服务器失败",
			zap.String("host", ldapConfig.Host),
			zap.Int("port", ldapConfig.Port),
			zap.Error(err),
		)
		return nil, fmt.Errorf("连接LDAP服务器失败: %w", err)
	}
	defer conn.Close()

	logger.Debug("LDAP服务器连接成功")

	// 绑定管理员用户
	if ldapConfig.BindDN != "" {
		if ldapConfig.BindPass == "" {
			logger.Warn("LDAP绑定密码为空，跳过管理员绑定",
				zap.String("bind_dn", ldapConfig.BindDN),
			)
		} else {
			logger.Debug("绑定LDAP管理员用户", zap.String("bind_dn", ldapConfig.BindDN))
			err = conn.Bind(ldapConfig.BindDN, ldapConfig.BindPass)
			if err != nil {
				logger.Error("LDAP管理员绑定失败",
					zap.String("bind_dn", ldapConfig.BindDN),
					zap.Error(err),
				)
				return nil, fmt.Errorf("LDAP管理员绑定失败: %w", err)
			}
			logger.Debug("LDAP管理员绑定成功")
		}
	} else {
		logger.Debug("跳过管理员绑定，使用匿名绑定")
	}

	// 搜索用户
	filter := ldapConfig.UserFilter
	if strings.TrimSpace(filter) == "" {
		filter = "(mobile=%s)"
	}
	searchFilter := fmt.Sprintf(filter, username)
	logger.Debug("搜索LDAP用户",
		zap.String("filter", searchFilter),
		zap.String("base_dn", ldapConfig.BaseDN),
		zap.String("search_by", "mobile"),
	)

	searchRequest := ldap.NewSearchRequest(
		ldapConfig.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		searchFilter,
		[]string{"dn", "uid", "mail", "cn", "displayName", "mobile"},
		nil,
	)

	sr, err := conn.Search(searchRequest)
	if err != nil {
		logger.Error("搜索LDAP用户失败",
			zap.String("username", username),
			zap.String("filter", searchFilter),
			zap.Error(err),
		)
		return nil, fmt.Errorf("搜索用户失败: %w", err)
	}

	logger.Debug("LDAP搜索完成",
		zap.Int("entries_found", len(sr.Entries)),
		zap.String("username", username),
	)

	if len(sr.Entries) != 1 {
		logger.Warn("用户搜索结果异常",
			zap.String("username", username),
			zap.Int("entries_count", len(sr.Entries)),
		)
		return nil, fmt.Errorf("用户不存在或找到多个用户")
	}

	userDN := sr.Entries[0].DN
	logger.Debug("找到用户DN",
		zap.String("username", username),
		zap.String("user_dn", userDN),
	)

	// 验证用户密码
	logger.Debug("验证用户密码")
	err = conn.Bind(userDN, password)
	if err != nil {
		logger.Warn("用户密码验证失败",
			zap.String("username", username),
			zap.String("user_dn", userDN),
			zap.Error(err),
		)
		return nil, fmt.Errorf("用户名或密码错误")
	}

	logger.Debug("用户密码验证成功")

	// 提取用户信息
	entry := sr.Entries[0]
	user := &LDAPUser{
		Username: entry.GetAttributeValue("uid"),
		Email:    entry.GetAttributeValue("mail"),
		FullName: entry.GetAttributeValue("cn"),
		Mobile:   entry.GetAttributeValue("mobile"),
	}

	if user.FullName == "" {
		user.FullName = entry.GetAttributeValue("displayName")
	}

	logger.Info("LDAP用户认证成功",
		zap.String("username", user.Username),
		zap.String("email", user.Email),
		zap.String("full_name", user.FullName),
	)

	return user, nil
}

// TestConnection 测试LDAP连接
func TestConnection() error {
	logger.Info("测试LDAP连接")

	if ldapConfig == nil {
		logger.Error("LDAP配置未初始化")
		return ErrLDAPNotConfigured
	}

	logger.Debug("连接LDAP服务器",
		zap.String("host", ldapConfig.Host),
		zap.Int("port", ldapConfig.Port),
	)

	conn, err := ldap.Dial("tcp", fmt.Sprintf("%s:%d", ldapConfig.Host, ldapConfig.Port))
	if err != nil {
		logger.Error("连接LDAP服务器失败",
			zap.String("host", ldapConfig.Host),
			zap.Int("port", ldapConfig.Port),
			zap.Error(err),
		)
		return fmt.Errorf("连接LDAP服务器失败: %w", err)
	}
	defer conn.Close()

	logger.Debug("LDAP服务器连接成功")

	if ldapConfig.BindDN != "" {
		logger.Debug("绑定LDAP管理员用户", zap.String("bind_dn", ldapConfig.BindDN))
		err = conn.Bind(ldapConfig.BindDN, ldapConfig.BindPass)
		if err != nil {
			logger.Error("LDAP管理员绑定失败",
				zap.String("bind_dn", ldapConfig.BindDN),
				zap.Error(err),
			)
			return fmt.Errorf("LDAP管理员绑定失败: %w", err)
		}
		logger.Debug("LDAP管理员绑定成功")
	}

	logger.Info("LDAP连接测试成功")
	return nil
}

// IsConfigured 返回LDAP是否已经配置
func IsConfigured() bool {
	return ldapConfig != nil
}
