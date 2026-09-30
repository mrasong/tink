# Tink

> **Self-hosted, macOS-first HTTP Notification Gateway**

Tink 是一个轻量、高效的自建消息通知网关，旨在将任意系统（CI/CD、监控、Shell 脚本、Cron、Webhook 等）发出的 HTTP 通知，以低延迟且可靠的方式推送到你的多台 Mac 设备。

---

## 🌟 核心特性

- **极简架构**：Go Server + bbolt (单文件嵌入式数据库) + SSE (Server-Sent Events) + Caddy。
- **天然多设备支持**：遵循 `User → Device → Connection` 设计，一条通知可自动分发到用户的所有 Mac (MacBook, iMac, Mac mini 等) 或定向投递。
- **离线可靠重放**：基于自增 `uint64` 消息 ID 与 `Last-Event-ID` 机制，断网重连后无损补发历史通知。
- **macOS Client (适配 macOS 15+ 与 macOS 26+)**：
  - SwiftUI `MenuBarExtra` 状态栏常驻。
  - **macOS 26+ Liquid Glass 效果**：磨砂玻璃与微折射流光高光渐变质感，并完美回退兼容 macOS 15+。
  - `UNUserNotificationCenter` 本地通知与点击自动唤醒目标 URL。
  - 具备指数退避带随机抖动的断线自动重连。
  - 安全凭据存储：Keychain 管理 Token 与设备 UUID。
- **开箱即用 CLI**：提供 `tink send` 命令行工具快速推送。

---

## 📦 项目结构

```text
tinker/
├── VERSION                 # 统一发布版本号，构建脚本默认从这里读取
├── server/                # Go 后端服务
│   ├── cmd/server/        # 服务端启动入口
│   ├── internal/          # 核心业务逻辑
│   │   ├── api/           # REST API 处理器与中间件 (鉴权、限流、CORS)
│   │   ├── auth/          # Bearer Token 验证与哈希存储
│   │   ├── sse/           # SSE Hub 内存广播与心跳保活
│   │   ├── store/         # bbolt 消息存储与索引、离线重放、自动清理
│   │   └── web/           # Go Embed FS 嵌入层 (编译打包静态产物)
│   ├── web/               # Svelte Web 管理控制台工程 (Vite + TypeScript)
│   ├── go.mod             # 后端独立 Go 模块 (github.com/mrasong/tinker/server)
│   ├── go.sum
│   └── Dockerfile         # 服务端容器镜像构建
├── client/                # macOS 客户端工程 (Swift Package Manager)
│   ├── Package.swift
│   ├── App/               # MenuBarExtra 主应用
│   ├── MenuBar/           # 状态栏菜单与状态指示
│   ├── Models/            # 数据模型
│   ├── Network/           # REST API & SSE 客户端
│   ├── Notification/      # 系统通知管理
│   ├── Settings/          # 设置界面
│   ├── Storage/           # Keychain & 本地存储
│   └── UI/                # Liquid Glass 与视图组件
├── scripts/               # 辅助与打包脚本
│   ├── build_server.sh    # 编译 Go 服务端到 build/tink-server
│   ├── build_app.sh       # 打包 macOS Tink.app Bundle
│   └── send.sh            # 便捷消息推送脚本 (curl 封装)
├── justfile               # Just 命令运行配置
├── Taskfile.yml           # Task 命令运行配置
├── compose.yaml           # Docker Compose 部署配置
├── Caddyfile              # 自动 HTTPS 反向代理配置
└── docs/                  # 设计与实施文档
```

---

## ⚡ 快捷命令 (Just / Task)

如果你安装了 [`just`](https://github.com/casey/just) 或 [`task`](https://taskfile.dev/)，可以直接运行：

```bash
# 构建全部 (服务端与客户端)
just build        # 或 task build

# 启动服务端
just run-server   # 或 task run-server

# 运行 macOS 客户端
just run-client   # 或 task run-client

# 发送测试通知
just send title="Hello" body="World"   # 或 task send TITLE="Hello" BODY="World"

# 运行后端测试
just test         # 或 task test
```

版本号统一维护在仓库根目录的 `VERSION` 文件中。构建时的 Build ID 自动使用最近一次 Git 提交的 7 位短 hash，直接运行构建命令即可自动引用版本和提交信息：

```bash
./scripts/build_server.sh
./scripts/build_app.sh
./scripts/build_docker.sh
```

如需临时构建其它版本，仍可通过脚本第一个参数覆盖默认值，例如 `./scripts/build_server.sh 0.3.0`。App 的第二个参数仍可覆盖数字 Bundle Build；Server 的第二个参数可覆盖 Build ID。

App 的第三个参数可覆盖显示用的 Git Commit，例如 `./scripts/build_app.sh 0.3.0 2026091411 abc1234`。

---

## 🚀 服务端快速启动

### 1. 本地直接编译运行

```bash
# 方式 A：使用脚本编译
./scripts/build_server.sh

# 方式 B：手动编译
cd server && go build -o ../build/tink-server ./cmd/server && cd ..

# 启动服务端 (默认端口 5021，首次启动控制台会输出 Master Token)
./build/tink-server --port 5021 --data ./data
```

启动后可直接在浏览器打开 `http://localhost:5021/dashboard`（或直接访问 `http://localhost:5021`），输入 Master Token 即可进入 Svelte 管理控制台，可视化管理设备、Secret Key 与发起推送测试。

> **自定义 Dashboard 访问路径**：
> 服务端支持通过环境变量 `TINK_DASHBOARD_ROUTE` 或启动参数 `--dashboard-route` 自定义控制台 URL（例如设置为 `/my-panel` 或 `/admin`），避免使用默认路径提升安全性。一旦配置了自定义路由，根路径 `/` 与原 `/dashboard` 均直接返回 404，彻底杜绝路径泄露。

### 2. Docker Compose 部署 (推荐)

```bash
docker compose up -d
```

## 预构建镜像

发布版本时 CI 会推送多架构（amd64/arm64）镜像到 GHCR：

```bash
docker pull ghcr.io/mrasong/tink:latest   # 或版本号 tag，如 v0.9.21
```

---

## 🖥️ macOS 客户端编译与运行

```bash
# 使用打包脚本构建 Tink.app (推荐)
./scripts/build_app.sh

# 或者进入 client 目录直接通过 Swift PM 运行
cd client
swift run Tink
```

首次运行时在设置面板填写服务器地址与 Bearer Token 即可完成设备注册与实时连接。

---

## 📡 接口使用示例 (cURL)

### 1. 发送通知 (支持 Tink 桌面端与 Bark 移动端双推)

```bash
curl -X POST http://localhost:5021/api/v1/messages \
  -H "Authorization: Bearer sk-tink-your_token" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Database Backup",
    "body": "Daily backup completed successfully.",
    "url": "https://dashboard.example.com",
    "sound": "default",
    "devices": ["your-tink-device-id"],
    "bark_devices": ["your-bark-device-key"],
    "bark_params": {
      "level": "timeSensitive",
      "icon": "https://example.com/icon.png",
      "badge": 1
    }
  }'
```

> 注：已禁用无目标广播机制，`devices` (Tink 客户端) 与 `bark_devices` (Bark 客户端) 至少需指定一个。`bark_params` 为可选的 JSON 键值对映射，用于向上游 Bark 推送时补充 Bark 原生个性化设置（如 `level`, `icon`, `badge`, `copy`, `autoCopy` 等）。
> 在后台管理控制台开启 **Bark Relay** 并配置上游服务地址后，所有传给 `bark_devices` 的设备将自动向上游 Bark-Server 转发；同时，在 iOS Bark App 中将服务器地址填写为 `http://your-domain:5021/bark-relay` 即可实现完整的 Bark 原生透明代理（包含注册获取 `device_key` 与原生推送）。

### 2. 心跳与服务检查 (Ping)

```bash
curl http://localhost:5021/api/v1/ping
```

---

## 📄 License

MIT
