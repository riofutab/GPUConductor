package cmd

import (
	"GPUConductor/internal/agent"
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
		config := &agent.Config{
			ServerURL: viper.GetString("agent.server"),
			NodeName:  viper.GetString("agent.name"),
			Tags:      viper.GetStringSlice("agent.tags"),
		}

		if config.ServerURL == "" {
			config.ServerURL = "http://localhost:8080"
		}
		if config.NodeName == "" {
			hostname, _ := os.Hostname()
			config.NodeName = hostname
		}

		agent := agent.New(config)
		log.Printf("启动GPUConductor代理节点: %s", config.NodeName)
		log.Printf("连接服务器: %s", config.ServerURL)

		if err := agent.Start(); err != nil {
			log.Fatal("代理节点启动失败:", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(agentCmd)
	agentCmd.Flags().StringP("server", "s", "http://localhost:8080", "主服务器地址")
	agentCmd.Flags().StringP("name", "n", "", "节点名称 (默认使用主机名)")
	agentCmd.Flags().StringSliceP("tags", "t", []string{}, "节点标签")

	viper.BindPFlag("agent.server", agentCmd.Flags().Lookup("server"))
	viper.BindPFlag("agent.name", agentCmd.Flags().Lookup("name"))
	viper.BindPFlag("agent.tags", agentCmd.Flags().Lookup("tags"))
}
