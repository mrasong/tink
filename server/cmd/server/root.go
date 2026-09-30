package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mrasong/tink/server/internal/store"
	"github.com/spf13/cobra"
)

var (
	globalDataDir string
)

var rootCmd = &cobra.Command{
	Use:   "tink-server",
	Short: "Tink Server - Self-hosted macOS notification & message gateway",
	Long: `Tink Server is a lightweight, self-hosted message and push gateway for macOS clients.
Supports real-time SSE channels, multi secret keys, embedded dashboard, and CLI management.`,
}

func init() {
	defaultDataDir := "./data"
	if envData := os.Getenv("TINK_DATA_DIR"); envData != "" {
		defaultDataDir = envData
	}

	rootCmd.PersistentFlags().StringVarP(&globalDataDir, "data", "d", defaultDataDir, "Directory to store tink.db")

	// 注册子命令
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(keyCmd)
	rootCmd.AddCommand(deviceCmd)
	rootCmd.AddCommand(versionCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// openStore 辅助打开数据库，若目录不存在则自动创建
func openStore() (*store.Store, error) {
	if err := os.MkdirAll(globalDataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data dir %s: %w", globalDataDir, err)
	}
	dbPath := filepath.Join(globalDataDir, "tink.db")
	return store.Open(dbPath)
}
