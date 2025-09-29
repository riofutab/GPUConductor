package cmd

import (
	"GPUConductor/internal/auth"
	"GPUConductor/internal/server"
	"log"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "启动GPUConductor主服务器",
	Long:  `启动主服务器，提供Web界面和任务调度功能`,
	Run: func(cmd *cobra.Command, args []string) {
		config := &server.Config{
			Port:     viper.GetString("server.port"),
			Database: viper.GetString("server.database"),
			Redis:    viper.GetString("server.redis"),
			LDAP: &auth.LDAPConfig{
				Host:     viper.GetString("ldap.host"),
				Port:     viper.GetInt("ldap.port"),
				BaseDN:   viper.GetString("ldap.base_dn"),
				UserDN:   viper.GetString("ldap.user_dn"),
				BindDN:   viper.GetString("ldap.bind_dn"),
				BindPass: viper.GetString("ldap.bind_pass"),
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
		if config.Database == "" {
			config.Database = "host=localhost user=gcond password=gcond dbname=gcond port=5432 sslmode=disable"
		}
		if config.Redis == "" {
			config.Redis = "localhost:6379"
		}
		if config.LDAP.Host == "" {
			config.LDAP.Host = "localhost"
		}
		if config.LDAP.Port == 0 {
			config.LDAP.Port = 389
		}
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
	serverCmd.Flags().StringP("database", "d", "", "PostgreSQL数据库连接字符串")
	serverCmd.Flags().StringP("redis", "r", "localhost:6379", "Redis连接地址")

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
	viper.BindPFlag("server.database", serverCmd.Flags().Lookup("database"))
	viper.BindPFlag("server.redis", serverCmd.Flags().Lookup("redis"))

	viper.BindPFlag("ldap.host", serverCmd.Flags().Lookup("ldap-host"))
	viper.BindPFlag("ldap.port", serverCmd.Flags().Lookup("ldap-port"))
	viper.BindPFlag("ldap.base_dn", serverCmd.Flags().Lookup("ldap-base-dn"))
	viper.BindPFlag("ldap.user_dn", serverCmd.Flags().Lookup("ldap-user-dn"))
	viper.BindPFlag("ldap.bind_dn", serverCmd.Flags().Lookup("ldap-bind-dn"))
	viper.BindPFlag("ldap.bind_pass", serverCmd.Flags().Lookup("ldap-bind-pass"))

	viper.BindPFlag("jwt.secret", serverCmd.Flags().Lookup("jwt-secret"))
	viper.BindPFlag("jwt.expiration_hours", serverCmd.Flags().Lookup("jwt-expiration"))
	viper.BindPFlag("jwt.refresh_expiration_days", serverCmd.Flags().Lookup("jwt-refresh-expiration"))
}
