package cmd

import (
	"GPUConductor/internal/auth"
	"GPUConductor/internal/server"
	"log"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "启动GPUConductor主服务器",
	Long:  `启动主服务器，提供Web界面和任务调度功能`,
	Run: func(cmd *cobra.Command, args []string) {
		cfgPort := viper.GetString("server.port")
		if cfgPort == "" && viper.IsSet("port") {
			cfgPort = viper.GetString("port")
		}
		if cfgPort == "" && viper.IsSet("server_port") {
			cfgPort = viper.GetString("server_port")
		}

		bindPass := strings.TrimSpace(viper.GetString("ldap.bind_pass"))
		if bindPass == "" {
			bindPass = strings.TrimSpace(viper.GetString("ldap.bind_password"))
		}

		heartbeatGrace := viper.GetInt("heartbeat_grace_seconds")
		if heartbeatGrace == 0 {
			heartbeatGrace = viper.GetInt("node.heartbeat_interval")
		}
		if heartbeatGrace == 0 {
			heartbeatGrace = 60
		}

		config := &server.Config{
			Port:                  cfgPort,
			HeartbeatGraceSeconds: heartbeatGrace,
			Database: &server.DatabaseConfig{
				Type:            viper.GetString("database.type"),
				Host:            viper.GetString("database.host"),
				Port:            viper.GetInt("database.port"),
				Name:            viper.GetString("database.name"),
				Username:        viper.GetString("database.username"),
				Password:        viper.GetString("database.password"),
				SSLMode:         viper.GetString("database.sslmode"),
				MaxOpenConns:    viper.GetInt("database.max_open_conns"),
				MaxIdleConns:    viper.GetInt("database.max_idle_conns"),
				ConnMaxLifetime: viper.GetInt("database.conn_max_lifetime"),
			},
			Redis: &server.RedisConfig{
				Host:     viper.GetString("redis.host"),
				Port:     viper.GetInt("redis.port"),
				Password: viper.GetString("redis.password"),
				DB:       viper.GetInt("redis.db"),
				PoolSize: viper.GetInt("redis.pool_size"),
			},
			LDAP: &auth.LDAPConfig{
				Host:     viper.GetString("ldap.host"),
				Port:     viper.GetInt("ldap.port"),
				BaseDN:   viper.GetString("ldap.base_dn"),
				UserDN:   viper.GetString("ldap.user_dn"),
				BindDN:   viper.GetString("ldap.bind_dn"),
				BindPass: bindPass,
			},
			JWT: &auth.JWTConfig{
				Secret:                viper.GetString("jwt.secret"),
				ExpirationHours:       viper.GetInt("jwt.expiration_hours"),
				RefreshExpirationDays: viper.GetInt("jwt.refresh_expiration_days"),
			},
		}

		// 设置默认值
		if config.Port == "" {
			config.Port = "8080"
		}

		// 数据库默认值
		if config.Database.Type == "" {
			config.Database.Type = "postgres"
		}
		if config.Database.Host == "" {
			config.Database.Host = "localhost"
		}
		if config.Database.Port == 0 {
			config.Database.Port = 5432
		}
		if config.Database.Name == "" {
			config.Database.Name = "gcond"
		}
		if config.Database.Username == "" {
			config.Database.Username = "gcond"
		}
		if config.Database.Password == "" {
			config.Database.Password = "gcond"
		}
		if config.Database.SSLMode == "" {
			config.Database.SSLMode = "disable"
		}
		if config.Database.MaxOpenConns == 0 {
			config.Database.MaxOpenConns = 25
		}
		if config.Database.MaxIdleConns == 0 {
			config.Database.MaxIdleConns = 25
		}
		if config.Database.ConnMaxLifetime == 0 {
			config.Database.ConnMaxLifetime = 300
		}

		// Redis默认值
		if config.Redis.Host == "" {
			config.Redis.Host = "localhost"
		}
		if config.Redis.Port == 0 {
			config.Redis.Port = 6379
		}
		if config.Redis.PoolSize == 0 {
			config.Redis.PoolSize = 10
		}

		// LDAP默认值
		if config.LDAP.Host == "" {
			config.LDAP.Host = "localhost"
		}
		if config.LDAP.Port == 0 {
			config.LDAP.Port = 389
		}

		// JWT默认值
		if config.JWT.Secret == "" {
			config.JWT.Secret = "your-secret-key-change-in-production"
		}
		if config.JWT.ExpirationHours == 0 {
			config.JWT.ExpirationHours = 24
		}
		if config.JWT.RefreshExpirationDays == 0 {
			config.JWT.RefreshExpirationDays = 7
		}

		srv := server.New(config)
		log.Printf("启动GPUConductor服务器，端口: %s", config.Port)
		if err := srv.Start(); err != nil {
			log.Fatal("服务器启动失败:", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)

	// 服务器配置
	serverCmd.Flags().StringP("port", "p", "8080", "服务器端口")

	// 数据库配置
	serverCmd.Flags().String("db-host", "localhost", "数据库主机")
	serverCmd.Flags().Int("db-port", 5432, "数据库端口")
	serverCmd.Flags().String("db-name", "gcond", "数据库名称")
	serverCmd.Flags().String("db-username", "gcond", "数据库用户名")
	serverCmd.Flags().String("db-password", "gcond", "数据库密码")
	serverCmd.Flags().String("db-sslmode", "disable", "数据库SSL模式")

	// Redis配置
	serverCmd.Flags().String("redis-host", "localhost", "Redis主机")
	serverCmd.Flags().Int("redis-port", 6379, "Redis端口")
	serverCmd.Flags().String("redis-password", "", "Redis密码")
	serverCmd.Flags().Int("redis-db", 0, "Redis数据库编号")

	// LDAP配置
	serverCmd.Flags().String("ldap-host", "localhost", "LDAP服务器地址")
	serverCmd.Flags().Int("ldap-port", 389, "LDAP服务器端口")
	serverCmd.Flags().String("ldap-base-dn", "", "LDAP Base DN")
	serverCmd.Flags().String("ldap-user-dn", "", "LDAP用户DN模板")
	serverCmd.Flags().String("ldap-bind-dn", "", "LDAP绑定DN")
	serverCmd.Flags().String("ldap-bind-pass", "", "LDAP绑定密码")

	// JWT配置
	serverCmd.Flags().String("jwt-secret", "", "JWT密钥")
	serverCmd.Flags().Int("jwt-expiration", 24, "JWT过期时间(小时)")
	serverCmd.Flags().Int("jwt-refresh-expiration", 7, "JWT刷新令牌过期时间(天)")

	// 绑定配置
	viper.BindPFlag("server.port", serverCmd.Flags().Lookup("port"))

	// 数据库配置绑定
	viper.BindPFlag("database.host", serverCmd.Flags().Lookup("db-host"))
	viper.BindPFlag("database.port", serverCmd.Flags().Lookup("db-port"))
	viper.BindPFlag("database.name", serverCmd.Flags().Lookup("db-name"))
	viper.BindPFlag("database.username", serverCmd.Flags().Lookup("db-username"))
	viper.BindPFlag("database.password", serverCmd.Flags().Lookup("db-password"))
	viper.BindPFlag("database.sslmode", serverCmd.Flags().Lookup("db-sslmode"))

	// Redis配置绑定
	viper.BindPFlag("redis.host", serverCmd.Flags().Lookup("redis-host"))
	viper.BindPFlag("redis.port", serverCmd.Flags().Lookup("redis-port"))
	viper.BindPFlag("redis.password", serverCmd.Flags().Lookup("redis-password"))
	viper.BindPFlag("redis.db", serverCmd.Flags().Lookup("redis-db"))

	// LDAP配置绑定
	viper.BindPFlag("ldap.host", serverCmd.Flags().Lookup("ldap-host"))
	viper.BindPFlag("ldap.port", serverCmd.Flags().Lookup("ldap-port"))
	viper.BindPFlag("ldap.base_dn", serverCmd.Flags().Lookup("ldap-base-dn"))
	viper.BindPFlag("ldap.user_dn", serverCmd.Flags().Lookup("ldap-user-dn"))
	viper.BindPFlag("ldap.bind_dn", serverCmd.Flags().Lookup("ldap-bind-dn"))
	viper.BindPFlag("ldap.bind_pass", serverCmd.Flags().Lookup("ldap-bind-pass"))

	// JWT配置绑定
	viper.BindPFlag("jwt.secret", serverCmd.Flags().Lookup("jwt-secret"))
	viper.BindPFlag("jwt.expiration_hours", serverCmd.Flags().Lookup("jwt-expiration"))
	viper.BindPFlag("jwt.refresh_expiration_days", serverCmd.Flags().Lookup("jwt-refresh-expiration"))
}
