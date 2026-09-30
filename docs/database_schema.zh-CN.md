# Tink 数据库存储结构文档

[English](database_schema.md) | [简体中文](database_schema.zh-CN.md)

> **版本**：v0.9.30

Tink 服务端基于嵌入式键值数据库 **bbolt**（单文件 `<data>/tink.db`，权限 `0600`，打开超时 3 秒）构建，采用高效的二进制前缀索引与 O(1) 哈希辅助索引设计，无需任何外部数据库。

布尔标识按层各有约定：桶内实体使用 `uint8` 的 `0/1`（如 `SecretKey.enabled`）；`meta` 值为纯字符串（`"true"`/`"false"`）；Settings API 交互则使用真正的 JSON 布尔。

---

## 1. Bucket 架构总览

Secret Key 扁平化架构已移除冗余的 `users` 实体，系统以 Secret Key (SK) 与 Device 为核心实体。打开数据库时共创建 9 个桶：

```text
tink.db
├── 核心数据存储桶
│   ├── secret_keys           # 认证密钥实体 (按 Key ID 索引)
│   ├── sk_hashes             # 密钥 SHA-256 哈希辅助索引 (O(1) 认证鉴权)
│   ├── devices               # 客户端设备实体 (直接绑定 key_id)
│   ├── messages              # 消息通知实体 (自增 uint64 大端序键)
│   ├── sessions              # 浏览器会话令牌 (Cookie 鉴权)
│   └── meta                  # 扁平全局配置 (bark_relay_enabled 等)
└── 二级有序索引桶 (范围检索)
    ├── idx_key_messages      # 密钥消息时间线索引 (write-only，暂无读方)
    ├── idx_device_messages   # 设备消息索引 (驱动 SSE 离线重放)
    └── idx_group_messages    # 分组消息索引 (write-only，暂无读方)
```

---

## 2. 核心数据存储桶详情

### 2.1 `secret_keys`（密钥数据桶）

- **Key**：`key_id`（`string`，如 `"admin"` 或 `"sk_3WHyWNzJ"`）
- **Value**：JSON 编码的 `SecretKey` 实体

```json
{
  "id": "sk_3WHyWNzJ",
  "name": "GitHub Actions CI",
  "role": "user",
  "hash": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
  "enabled": 1,
  "created_at": 1789258500000,
  "last_used": 1789258800000
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | `string` | 密钥唯一 ID（`admin`，或 `sk_` + 原始密钥末 8 位） |
| `name` | `string` | 用途备注（如 "GitHub Actions"、"Grafana"、"Work Mac"） |
| `role` | `string` | `"admin"`（受系统保护，不可禁用/删除）或 `"user"`（普通业务发信密钥） |
| `hash` | `string` | 原始密钥明文的 SHA-256 十六进制哈希（数据库不存明文） |
| `enabled` | `uint8` | `1` 启用 / `0` 禁用（禁用后鉴权直接拦截） |
| `created_at` | `int64` 毫秒 | 创建时间 |
| `last_used` | `int64` 毫秒 | 最近一次成功调用 API 的时间（为 `0` 时省略） |

`GetKeyByHash` 会在每次鉴权成功后刷新 `last_used`，并对历史上以哈希为 Key 的旧行做惰性迁移。

### 2.2 `sk_hashes`（密钥哈希辅助索引桶）

- **Key**：`SHA-256(rawToken)` 的十六进制字符串
- **Value**：对应的 `key_id`（`string`）

请求携带 `Authorization: Bearer sk-tink-...` 时，服务端计算哈希后在本桶 O(1) 反查 `key_id`，再加载完整实体做权限判定，鉴权路径零遍历。与 `secret_keys` 在同一 bbolt 事务中双写/双删。

### 2.3 `devices`（设备数据桶）

- **Key**：`device_id`（`string`，由 macOS 客户端生成的持久化设备 UUID）
- **Value**：JSON 编码的 `Device` 实体

```json
{
  "id": "dev-macbook-pro",
  "key_id": "sk_3WHyWNzJ",
  "name": "MacBook Pro 16\"",
  "created_at": 1789258530000,
  "last_connected_at": 1789258530000,
  "last_disconnected_at": 1789258800000
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | `string` | 设备唯一标识 UUID |
| `key_id` | `string` | 注册/绑定该设备的 Secret Key ID（普通 SK 的数据隔离依据） |
| `name` | `string` | 设备友好名称 |
| `created_at` | `int64` 毫秒 | 首次注册时间 |
| `last_connected_at` | `int64` 毫秒 | 最近一次建立 SSE 连接的时间 |
| `last_disconnected_at` | `int64` 毫秒 | 最近一次断开 SSE 连接的时间，`0` 表示从未断开 |

在线状态不落库：API 响应中的 `status` 由 SSE Hub 按当前连接运行时计算（`0` 离线 / `1` 在线）。注册为 upsert——重复注册保留原有 `key_id` 归属与时间戳。

### 2.4 `messages`（消息数据桶）

- **Key**：8 字节大端序 `Itob(message_id)`，因此在 bbolt B+ 树中天然按时间严格递增存放
- **Value**：JSON 编码的 `Message` 实体

```json
{
  "id": 10024,
  "key_id": "sk_3WHyWNzJ",
  "devices": ["dev-macbook-pro"],
  "group": "deploy",
  "title": "部署成功",
  "body": "v1.2.0 已部署至生产集群并完成健康检查",
  "url": "https://dashboard.example.com",
  "sound": "default",
  "payload": { "env": "production", "git_sha": "7abecb3" },
  "created_at": 1789258800000,
  "expires_at": 1789863600000
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | `uint64` | 全局单调自增序列号（`bbolt.NextSequence()`） |
| `key_id` | `string` | 发送该消息的 Secret Key ID |
| `devices` | `[]string` | 目标设备 ID 列表——仅含 Tink 目标的推送才会入库；纯 Bark 推送不写本桶 |
| `group` | `string` | 通知分组标识（如 "deploy"，用于客户端归类聚合） |
| `title` / `body` | `string` | 通知标题 / 正文 |
| `url` | `string` | 点击通知后唤起的跳转链接 |
| `sound` | `string` | 提示音名称（`default`、`glass`、`silent` 等） |
| `payload` | `object` | 扩展透传键值对 |
| `created_at` | `int64` 毫秒 | 发送入库时间戳 |
| `expires_at` | `int64` 毫秒 | 过期时间（默认 `created_at + 7 天`） |

### 2.5 `sessions`（浏览器会话桶）

随控制台登录流程引入的 Cookie 会话（`tink_session`）。

- **Key**：原始会话令牌的 SHA-256 十六进制（登录时下发的 64 位十六进制随机串）
- **Value**：JSON `Session` 实体

```json
{
  "token_hash": "9a1f…",
  "key_id": "admin",
  "created_at": 1789258500000,
  "last_seen": 1789258800000,
  "absolute_exp": 1789272900000
}
```

生命周期：**空闲 8 小时**（经 `last_seen` 跟踪，最多每 60 秒刷新一次）＋**绝对 24 小时**（`absolute_exp`）。过期会话在查询时惰性删除，并由后台清理协程每小时批量回收；logout 立即物理删除。

### 2.6 `meta`（系统全局配置桶）

扁平字符串 Key/Value（当前为 Bark Relay 三件套）：

| Key | 值示例 | 说明 |
|---|---|---|
| `bark_relay_enabled` | `"true"` / `"false"` | 是否开启 Bark 双推转发与透明代理（读取端兼容 `"1"`） |
| `bark_server_url` | `"https://api.day.app"` | 上游 Bark-Server 地址（支持自建）；写入去尾部 `/`，置空回落默认值 |
| `bark_route_path` | `"/bark-relay"` | 本地反向代理路由前缀（iOS Bark App 中作为服务器地址填入）；规范化补前导 `/`，空或 `/` 回落 `/bark-relay` |

默认值：Relay **关闭**、`https://api.day.app`、`/bark-relay`。

---

## 3. 二级索引桶设计（离线重放与范围检索）

用途：macOS 客户端断网重连、合盖休眠唤醒后，凭 `Last-Event-ID` 无损补发离线期间的通知。

### 3.1 `idx_device_messages`（设备专属消息索引——重放主力）

- **Key 构造**：`{device_id}:` + `Itob(message_id)`（后 8 字节为大端序 uint64）
- **Value**：`Itob(message_id)`
- **检索方式**：设备重连携带 `Last-Event-ID: afterID` 时，Cursor 寻址到 `{device_id}:{Itob(afterID + 1)}` 沿前缀正向扫描，并按 `now < expires_at` 过滤，复杂度 O(log N + K)。SSE 单次重放上限 **200** 条。目前唯一有读路径的索引。

### 3.2 `idx_key_messages` / 3.3 `idx_group_messages`

编码同上，前缀为 `{key_id}:` / `{group}:`。与消息同事务写入；预留给按密钥审计与按分组聚合查询——**当前尚无接口读取**。

---

## 4. 数据一致性与自动垃圾回收 (GC)

### 4.1 事务不变量

- 密钥增删改：`secret_keys` 与 `sk_hashes` 在同一 bbolt 事务中双写，哈希索引与实体永不背离。
- 消息入库：`messages` 与三个索引桶在同一事务中原子写入。
- 设备重注册：保留原 `key_id` 归属，严格保障密钥间设备隔离。

### 4.2 后台清理协程（每小时 + 启动时）

- 游标扫描 `messages`，`now > expires_at` 的记录从主桶与全部索引桶级联删除；
- 同步清扫 `sessions` 中的过期会话。

防止 bbolt 单文件因死消息与陈旧会话长期积累而膨胀，保障读写性能。

## 5. 相关文档

- REST 接口：[apidoc.zh-CN.md](apidoc.zh-CN.md) / [English](apidoc.md)
- 项目总览：[README.zh-CN.md](../README.zh-CN.md) / [English](../README.md)
