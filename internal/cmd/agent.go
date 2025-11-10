package cmd

import (
	"GPUConductor/internal/agent"
	"GPUConductor/internal/config"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "启动GPUConductor代理节点",
	Long:  `启动代理节点，监控本机GPU状态并执行任务`,
	Run: func(cmd *cobra.Command, args []string) {
		cfgPath, _ := cmd.Flags().GetString("config")
		if cfgPath == "" {
			cfgPath = viper.GetString("config")
		}
		if cfgPath == "" {
			log.Fatal("请使用 --config 指定配置文件")
		}

		cfg, err := config.LoadConfig(cfgPath)
		if err != nil {
			log.Fatalf("加载配置失败: %v", err)
		}

		if cfg.Node.ServerURL == "" {
			cfg.Node.ServerURL = "http://localhost:8080"
		}
		if cfg.Node.NodeName == "" {
			hostname, _ := os.Hostname()
			cfg.Node.NodeName = hostname
		}

		agent := agent.New(cfg)
		log.Printf("启动GPUConductor代理节点: %s", cfg.NodeName)
		log.Printf("连接服务器: %s", cfg.ServerURL)

		if err := agent.Start(); err != nil {
			log.Fatal("代理节点启动失败:", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(agentCmd)
	agentCmd.Flags().StringP("config", "c", "", "配置文件路径")
}
