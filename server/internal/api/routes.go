package api

import (
	"github.com/mrasong/tink/server/internal/i18n"
	"net/http"
	"strings"
	"time"

	"github.com/mrasong/tink/server/internal/sse"
	"github.com/mrasong/tink/server/internal/store"
	"github.com/mrasong/tink/server/internal/web"
)

// Config 服务端全局 API 与路由配置
type Config struct {
	EnableDashboard bool
	DashboardRoute  string
}

// DefaultConfig 默认配置 (默认开启 Dashboard，默认路径 /dashboard)
func DefaultConfig() Config {
	return Config{
		EnableDashboard: true,
		DashboardRoute:  "/dashboard",
	}
}

// NormalizeDashboardRoute 规范化 Dashboard 路由路径 (例如: "admin" -> "/admin", "/admin/" -> "/admin")
func NormalizeDashboardRoute(route string) string {
	clean := strings.TrimSpace(route)
	if clean == "" {
		return "/dashboard"
	}
	if clean == "/" {
		return "/"
	}
	clean = "/" + strings.Trim(clean, "/")
	return clean
}

// RegisterRoutes 设置系统全局 HTTP 路由及中间件链
func RegisterRoutes(mux *http.ServeMux, s *store.Store, h *sse.Hub, cfgs ...Config) http.Handler {
	cfg := DefaultConfig()
	if len(cfgs) > 0 {
		cfg = cfgs[0]
		if cfg.DashboardRoute == "" {
			cfg.DashboardRoute = "/dashboard"
		}
	}
	dashRoute := NormalizeDashboardRoute(cfg.DashboardRoute)

	handler := NewHandler(s, h)
	h.SetDisconnectHandler(func(deviceID string) { _ = s.UpdateDeviceConnected(deviceID, false) })

	// 限流器: 凭证维度 120 次/分钟；鉴权失败维度按 IP 20 次/分钟 (防凭证爆破绕过限流)
	rateLimiter := NewRateLimiter(120, time.Minute)
	authFailLimiter := NewAuthFailLimiter(20, time.Minute)

	// 公开端点：心跳探测
	mux.HandleFunc("/api/v1/ping", handler.Ping)

	// 鉴权包装函数：失败限流在最外层，使 401 请求也计入 IP 额度
	authWrap := func(f http.HandlerFunc) http.Handler {
		return AuthFailLimitMiddleware(authFailLimiter)(AuthMiddleware(s)(RateLimitMiddleware(rateLimiter)(f)))
	}

	// 控制台登录：用 Secret Key 换 httpOnly 会话 cookie；仅失败限流保护
	mux.Handle("/api/v1/login", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !cfg.EnableDashboard && !isUDSRequest(r) {
			JSONError(w, r, http.StatusNotFound, i18n.MsgEndpointNotFound)
			return
		}
		AuthFailLimitMiddleware(authFailLimiter)(http.HandlerFunc(handler.Login)).ServeHTTP(w, r)
	}))
	mux.Handle("/api/v1/logout", authWrap(handler.Logout))

	// 身份与权限查询 (支持 Admin 与普通 SK)
	mux.Handle("/api/v1/me", authWrap(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			handler.GetMe(w, r)
		} else {
			JSONError(w, r, http.StatusMethodNotAllowed, i18n.MsgMethodNotAllowed)
		}
	}))

	// 核心 REST API - 设备管理
	mux.Handle("/api/v1/devices", authWrap(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			handler.RegisterDevice(w, r)
		case http.MethodGet:
			handler.ListDevices(w, r)
		default:
			JSONError(w, r, http.StatusMethodNotAllowed, i18n.MsgMethodNotAllowed)
		}
	}))

	mux.Handle("/api/v1/devices/", authWrap(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			handler.DeleteDevice(w, r)
		} else {
			JSONError(w, r, http.StatusMethodNotAllowed, i18n.MsgMethodNotAllowed)
		}
	}))

	// 消息推送接口
	mux.Handle("/api/v1/messages", authWrap(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			handler.SendMessage(w, r)
		} else {
			JSONError(w, r, http.StatusMethodNotAllowed, i18n.MsgMethodNotAllowed)
		}
	}))

	// Admin 专用密钥管理包装函数
	adminAuthWrap := func(f http.HandlerFunc) http.Handler {
		return AuthFailLimitMiddleware(authFailLimiter)(AuthMiddleware(s)(RateLimitMiddleware(rateLimiter)(RequireAdminRole()(f))))
	}

	keysHandler := adminAuthWrap(func(w http.ResponseWriter, r *http.Request) {
		// 路由到具体的 key 操作: /api/v1/keys 或 /api/v1/tokens
		cleanPath := strings.TrimSuffix(r.URL.Path, "/")
		if cleanPath == "/api/v1/keys" || cleanPath == "/api/v1/tokens" {
			switch r.Method {
			case http.MethodGet:
				handler.ListKeys(w, r)
			case http.MethodPost:
				handler.CreateKey(w, r)
			default:
				JSONError(w, r, http.StatusMethodNotAllowed, i18n.MsgMethodNotAllowed)
			}
			return
		}

		// /api/v1/keys/:id 或 /api/v1/tokens/:id
		if strings.HasPrefix(r.URL.Path, "/api/v1/keys/") || strings.HasPrefix(r.URL.Path, "/api/v1/tokens/") {
			switch r.Method {
			case http.MethodPut:
				handler.UpdateKey(w, r)
			case http.MethodDelete:
				handler.DeleteKey(w, r)
			default:
				JSONError(w, r, http.StatusMethodNotAllowed, i18n.MsgMethodNotAllowed)
			}
			return
		}

		JSONError(w, r, http.StatusNotFound, i18n.MsgEndpointNotFound)
	})

	registerKeyRoutes := func(base string) {
		mux.Handle(base, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cfg.EnableDashboard && !isUDSRequest(r) {
				JSONError(w, r, http.StatusNotFound, i18n.MsgEndpointNotFound)
				return
			}
			keysHandler.ServeHTTP(w, r)
		}))

		mux.Handle(base+"/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cfg.EnableDashboard && !isUDSRequest(r) {
				JSONError(w, r, http.StatusNotFound, i18n.MsgEndpointNotFound)
				return
			}
			keysHandler.ServeHTTP(w, r)
		}))
	}

	// 注册 /api/v1/keys (同时保留 /api/v1/tokens 向后兼容)
	registerKeyRoutes("/api/v1/keys")
	registerKeyRoutes("/api/v1/tokens")

	// SSE 实时通道 (长连接不经过统一短请求限流器；失败限流只作用于握手被拒的请求)
	mux.Handle("/api/v1/events", AuthFailLimitMiddleware(authFailLimiter)(AuthMiddleware(s)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			handler.EventsSSE(w, r)
		} else {
			JSONError(w, r, http.StatusMethodNotAllowed, i18n.MsgMethodNotAllowed)
		}
	}))))

	// Admin 专用系统设置接口 (/api/v1/settings)
	mux.Handle("/api/v1/settings", adminAuthWrap(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handler.GetSettings(w, r)
		case http.MethodPut:
			handler.UpdateSettings(w, r)
		default:
			JSONError(w, r, http.StatusMethodNotAllowed, i18n.MsgMethodNotAllowed)
		}
	}))

	// Bark 透明反向代理处理器
	barkProxyHandler := BarkRelayProxy(s)

	// 顶层挂载日志与安全头中间件
	// 注：原全局 Access-Control-Allow-Origin: * 已移除。控制台与 API 经 Caddy 同源访问，
	// macOS/CLI 等原生客户端不受 CORS 约束；如有浏览器第三方集成需求再按白名单加回。
	return LoggerMiddleware(SecurityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// 检查是否命中 Bark Relay 透明代理路由
		if settings, err := s.GetSettings(); err == nil {
			routePath := settings.BarkRoutePath
			if routePath != "" && (r.URL.Path == routePath || strings.HasPrefix(r.URL.Path, routePath+"/")) {
				if !settings.BarkRelayEnabled {
					JSONError(w, r, http.StatusNotFound, i18n.MsgEndpointNotFound)
					return
				}
				barkProxyHandler.ServeHTTP(w, r)
				return
			}
		}

		// 路由末尾斜杠兼容处理
		if strings.HasPrefix(r.URL.Path, "/api/v1/devices/") ||
			strings.HasPrefix(r.URL.Path, "/api/v1/keys/") ||
			strings.HasPrefix(r.URL.Path, "/api/v1/tokens/") {
			mux.ServeHTTP(w, r)
			return
		}

		// Web 控制台路由分发 (动态支持自定义 DashboardRoute)
		if !cfg.EnableDashboard {
			// 控制台未启用时，Web 页面路径返回 404
			if dashRoute == "/" && r.URL.Path == "/" {
				http.NotFound(w, r)
				return
			}
			if dashRoute != "/" && (r.URL.Path == "/" || strings.HasPrefix(r.URL.Path, dashRoute)) {
				http.NotFound(w, r)
				return
			}
			mux.ServeHTTP(w, r)
			return
		}

		// 1. 若配置为根路径 "/"
		if dashRoute == "/" {
			// 若不是 API 请求，则直接交由 web.Handler 处理
			if !strings.HasPrefix(r.URL.Path, "/api/") {
				web.Handler().ServeHTTP(w, r)
				return
			}
		} else {
			// 2. 若配置为独立子路径 (如自定义路径 "/secret-admin" 或默认 "/dashboard")
			// 仅当保持默认 "/dashboard" 且未自定义隐蔽路径时，保留根路径友好跳转；
			// 一旦自定义了非 "/dashboard" 路由，根路径直接返回 404，彻底避免暴露私密路径。
			if r.URL.Path == "/" {
				if dashRoute == "/dashboard" {
					http.Redirect(w, r, "/dashboard/", http.StatusFound)
				} else {
					http.NotFound(w, r)
				}
				return
			}

			// 规范化缺少末尾斜杠的请求: /admin -> /admin/
			if r.URL.Path == dashRoute {
				http.Redirect(w, r, dashRoute+"/", http.StatusMovedPermanently)
				return
			}

			// 匹配子路径前缀并分发静态资源
			if strings.HasPrefix(r.URL.Path, dashRoute+"/") {
				http.StripPrefix(dashRoute, web.Handler()).ServeHTTP(w, r)
				return
			}
		}

		mux.ServeHTTP(w, r)
	})))
}
