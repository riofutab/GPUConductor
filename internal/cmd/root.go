package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// 版本信息
var (
	version   = "dev"
	commit    = "unknown"
	buildTime = "unknown"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "gcond",
	Short: "GPUConductor - 分布式GPU任务调度系统",
	Long: `GPUConductor 是一个分布式GPU任务调度系统，支持：
- 多机器GPU监控
- 智能任务队列
- Web界面管理
- Docker容器执行`,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "显示版本信息",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("GPUConductor %s\n", version)
		fmt.Printf("Commit: %s\n", commit)
		fmt.Printf("Build Time: %s\n", buildTime)
	},
}

// SetVersionInfo 设置版本信息
func SetVersionInfo(v, c, bt string) {
	version = v
	commit = c
	buildTime = bt
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)
	
	rootCmd.AddCommand(versionCmd)
	
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "配置文件路径 (默认: $HOME/.gcond.yaml)")
	rootCmd.PersistentFlags().Bool("debug", false, "启用调试模式")
	
	viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName(".gcond")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "使用配置文件:", viper.ConfigFileUsed())
	}
}
