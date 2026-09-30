package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/mrasong/tink/server/internal/api"
	"github.com/mrasong/tink/server/internal/auth"
	"github.com/mrasong/tink/server/internal/sse"
	"github.com/mrasong/tink/server/internal/store"
	"github.com/spf13/cobra"
	bbolt "go.etcd.io/bbolt"
)

var (
	servePort            int
	serveInitialToken    string
	serveEnableDashboard bool
	serveDashboardRoute  string
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start HTTP and SSE push server gateway",
	Long:  `Runs the Tink notification server listening for incoming HTTP push requests and SSE connections.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runServe()
	},
}

func init() {
	defaultPort := 5021
	if envPort := os.Getenv("TINK_PORT"); envPort != "" {
		_, _ = fmt.Sscanf(envPort, "%d", &defaultPort)
	}

	defaultToken := os.Getenv("TINK_INITIAL_TOKEN")
	defaultDashboard := true
	if envDash := os.Getenv("TINK_ENABLE_DASHBOARD"); envDash == "false" || envDash == "0" {
		defaultDashboard = false
	}

	defaultDashboardRoute := "/dashboard"
	if envRoute := os.Getenv("TINK_DASHBOARD_ROUTE"); envRoute != "" {
		defaultDashboardRoute = envRoute
	}

	serveCmd.Flags().IntVarP(&servePort, "port", "p", defaultPort, "Server listening port")
	serveCmd.Flags().StringVar(&serveInitialToken, "token", defaultToken, "Initial master admin token for first setup (optional)")
	serveCmd.Flags().BoolVar(&serveEnableDashboard, "enable-dashboard", defaultDashboard, "Enable embedded web dashboard and token management APIs")
	serveCmd.Flags().StringVar(&serveDashboardRoute, "dashboard-route", defaultDashboardRoute, "Custom URL route path for web dashboard (default \"/dashboard\")")

	// 将 serve 常用 flags 同样绑定在 rootCmd 上，使得直接 `tink-server -p 9000` 无缝向下兼容
	rootCmd.Flags().IntVarP(&servePort, "port", "p", defaultPort, "Server listening port")
	rootCmd.Flags().StringVar(&serveInitialToken, "token", defaultToken, "Initial master admin token for first setup (optional)")
	rootCmd.Flags().BoolVar(&serveEnableDashboard, "enable-dashboard", defaultDashboard, "Enable embedded web dashboard and token management APIs")
	rootCmd.Flags().StringVar(&serveDashboardRoute, "dashboard-route", defaultDashboardRoute, "Custom URL route path for web dashboard (default \"/dashboard\")")
}

func runServe() error {
	if err := os.MkdirAll(globalDataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data dir: %w", err)
	}

	dbPath := filepath.Join(globalDataDir, "tink.db")
	st, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open bbolt database: %w", err)
	}
	defer st.Close()

	// 检查或生成初始 Admin Secret Key
	// 安全策略：仅当库中尚无任何密钥时才注册/生成，避免每次启动重建 admin key
	// (旧实现每次启动都以下面写死的 ID 注册，被删除的 key 会复活)
	hasKey := false
	_ = st.DB().View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(store.BucketSecretKeys)
		if b != nil {
			if k, _ := b.Cursor().First(); k != nil {
				hasKey = true
			}
		}
		return nil
	})

	if !hasKey {
		rawToken := serveInitialToken
		if rawToken == "" {
			var err error
			rawToken, err = auth.GenerateToken()
			if err != nil {
				return fmt.Errorf("failed to generate admin token: %w", err)
			}
		}
		adminKey := &store.SecretKey{
			ID:        "admin",
			Name:      "Admin Secret Key",
			Role:      "admin",
			Hash:      store.HashToken(rawToken),
			Enabled:   1,
			CreatedAt: time.Now().UnixMilli(),
		}
		if err := st.CreateKey(adminKey); err != nil {
			return fmt.Errorf("failed to create admin key: %w", err)
		}
		log.Printf("\n=======================================================\n[Security] Generated Admin Secret Key: %s\nPlease save this key for client and dashboard login!\n=======================================================\n", rawToken)
	} else if serveInitialToken != "" {
		log.Println("[Security] TINK_INITIAL_TOKEN/--token ignored: admin key already provisioned")
	}

	hub := sse.NewHub()
	defer hub.CloseAll()

	// 后台定期清理过期消息与会话 (每 1 小时触发一次)
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	go func() {
		for range ticker.C {
			count, err := st.CleanExpiredMessages()
			if err != nil {
				log.Printf("[Cleaner] Expired messages cleanup failed: %v", err)
			} else if count > 0 {
				log.Printf("[Cleaner] Cleaned %d expired messages", count)
			}
			sessCount, err := st.CleanExpiredSessions()
			if err != nil {
				log.Printf("[Cleaner] Expired sessions cleanup failed: %v", err)
			} else if sessCount > 0 {
				log.Printf("[Cleaner] Cleaned %d expired sessions", sessCount)
			}
		}
	}()

	mux := http.NewServeMux()
	handler := api.RegisterRoutes(mux, st, hub, api.Config{
		EnableDashboard: serveEnableDashboard,
		DashboardRoute:  serveDashboardRoute,
	})

	addr := fmt.Sprintf(":%d", servePort)
	server := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	go func() {
		dashStatus := "disabled"
		if serveEnableDashboard {
			cleanRoute := api.NormalizeDashboardRoute(serveDashboardRoute)
			dashStatus = fmt.Sprintf("http://localhost:%d%s", servePort, cleanRoute)
		}
		log.Printf("[Server] Tink Server listening on http://0.0.0.0%s (dashboard: %s)", addr, dashStatus)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server listen failed: %v", err)
		}
	}()

	// 启动 UNIX Domain Socket 服务，供本地 CLI 和管理通信
	socketPath := filepath.Join(globalDataDir, "tink.sock")
	_ = os.Remove(socketPath) // 清理旧 socket 文件
	udsListener, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Printf("[Server] Warning: failed to listen on unix socket %s: %v", socketPath, err)
	} else {
		_ = os.Chmod(socketPath, 0600)
		udsServer := &http.Server{
			// 仅此 listener 注入 UDS 标记；鉴权不再嗅探 RemoteAddr 字符串
			Handler: api.UDSMarkerMiddleware(handler),
		}
		go func() {
			log.Printf("[Server] Tink Server listening on unix socket: %s", socketPath)
			if err := udsServer.Serve(udsListener); err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, net.ErrClosed) {
				log.Printf("[Server] Unix socket server error: %v", err)
			}
		}()
		defer func() {
			_ = udsServer.Close()
			_ = os.Remove(socketPath)
		}()
	}

	// 监听优雅退出信号
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)
	<-stopChan

	log.Println("[Server] Shutting down gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
	log.Println("[Server] Stopped.")
	return nil
}
