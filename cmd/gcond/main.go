package main

import (
	"GPUConductor/internal/cmd"
	"fmt"
	"os"
)

// 版本信息，在构建时通过 ldflags 注入
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

func main() {
	// 设置版本信息
	cmd.SetVersionInfo(Version, Commit, BuildTime)
	
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}