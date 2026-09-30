# Tink 数据库存储结构文档 (Database Schema)

Tink 服务端基于嵌入式键值数据库 **`bbolt`** (单文件数据库 `tink.db`) 构建，采用高效的二进制前缀索引与 $O(1)$ 哈希辅助索引设计。

---

## 1. Bucket 架构总览

在全新的 Secret Key 扁平化架构下，移除了冗余的 `users` 实体，系统以 Secret Key (SK) 与 Device 为核心实体，所有布尔标识均采用 `0 / 1 (uint8)` 存储：

```text
tink.db
├── 核心数据存储桶
│   ├── secret_keys           # 认证密钥实体 (按 Key ID 索引)
│   ├── sk_hashes             # 密钥 SHA-256 哈希辅助索引 (用于 O(1) 认证鉴权)
│   ├── devices               # 客户端设备实体 (直接绑定 key_id)
│   ├── messages              # 消息通知实体 (自增 uint64 大端序序列)
│   └── meta                  # 数据库元信息与全局系统配置 (扁平 KV，如 bark_relay_enabled 等)
└── 二级有序索引桶 (用于范围查找与离线消息重放)
    ├── idx_key_messages      # 密钥消息时间线索引 (key_id:msg_id)
    ├── idx_device_messages   # 设备消息索引 (device_id:msg_id)
    └── idx_group_messages    # 分组消息索引 (group:msg_id)
```

---

## 2. 核心数据存储桶详情

### 2.1 `secret_keys` (密钥数据桶)

- **Bucket 标识**：`secret_keys`
- **Key**：`key_id` (`string`，如 `"admin"` 或 `"sk_3WHyWNzJ"`)
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

| 字段         | 类型      | 说明                                                                                 |
| :----------- | :-------- | :----------------------------------------------------------------------------------- |
| `id`         | `string`  | 密钥唯一 ID (前缀 `sk_` 或 `admin`)                                                  |
| `name`       | `string`  | 密钥用途备注 (如 "GitHub Actions", "Grafana", "Work Mac")                            |
| `role`       | `string`  | 权限角色：`"admin"` (管理员，受系统保护不可删除/禁用) 或 `"user"` (普通业务发信密钥) |
| `hash`       | `string`  | 原始密钥明文的 SHA-256 十六进制哈希值 (数据库不存明文)                               |
| `enabled`    | `uint8`   | 是否启用：`1` 表示启用，`0` 表示禁用 (禁用后拦截所有接口请求)                        |
| `created_at` | `int64`（Unix 毫秒） | 创建时间                                                                             |
| `last_used`  | `int64`（Unix 毫秒） | 最近一次成功调用 API 的时间 (可选)                                                   |

---

### 2.2 `sk_hashes` (密钥哈希辅助索引桶)

- **Bucket 标识**：`sk_hashes`
- **Key**：`SHA-256(rawToken)` 的十六进制字符串 (`string`)
- **Value**：对应的 `key_id` (`string`)
- **设计作用**：外部调用传入 `Authorization: Bearer sk-tink-...` 时，服务端计算哈希后在本桶中进行 $O(1)$ 查找，秒级反查出 `key_id`，再获取完整实体进行权限与角色判定，彻底避免全局遍历扫描。

---

### 2.3 `devices` (设备数据桶)

- **Bucket 标识**：`devices`
- **Key**：`device_id` (`string`，由 macOS 客户端生成的唯一持久化硬件 UUID)
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

| 字段         | 类型      | 说明                                                                 |
| :----------- | :-------- | :------------------------------------------------------------------- |
| `id`         | `string`  | 设备唯一标识 UUID                                                    |
| `key_id`     | `string`  | 注册/绑定该设备的 Secret Key ID (用于普通 SK 的数据隔离)             |
| `name`       | `string`  | 设备友好名称 (如 "Work MacBook", "Home Mac mini")                    |
| `created_at` | `int64`（Unix 毫秒） | 首次注册时间                                                         |
| `last_connected_at` | `int64`（Unix 毫秒） | 最近一次建立 SSE 连接的时间 |
| `last_disconnected_at` | `int64`（Unix 毫秒） | 最近一次断开 SSE 连接的时间，`0` 表示从未断开 |

设备当前在线状态不写入数据库。API 响应中的 `status` 为运行时字段：`0` 表示离线，`1` 表示在线，由 SSE Hub 根据当前连接动态生成。

---

### 2.4 `messages` (消息数据桶)

- **Bucket 标识**：`messages`
- **Key**：8 字节大端序无符号整数 `Itob(message_id)` (`[]byte`)，天然保证在 bbolt B+ 树底层严格按时间递增序列存放。
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
  "payload": {
    "env": "production",
    "git_sha": "7abecb3"
  },
  "created_at": 1789258800000,
  "expires_at": 1789863600000
}
```

| 字段         | 类型       | 说明                                                                      |
| :----------- | :--------- | :------------------------------------------------------------------------ |
| `id`         | `uint64`   | 全局单调自增消息序列号 (基于 `bbolt.NextSequence()`)                      |
| `key_id`     | `string`   | 发送该消息的 Secret Key ID                                                |
| `devices`    | `[]string` | 目标设备 ID 列表 (**必填项**，已禁用无目标广播，必须指定接收此消息的设备) |
| `group`      | `string`   | 通知分组标识 (如 "deploy", "monitor"，用于客户端归类聚合)                 |
| `title`      | `string`   | 通知标题                                                                  |
| `body`       | `string`   | 通知正文                                                                  |
| `url`        | `string`   | 点击通知后自动在浏览器唤起的跳转链接 (可选)                               |
| `sound`      | `string`   | 通知提示音名称 (如 "default", "glass", "bell", "silent")                  |
| `payload`    | `object`   | 扩展透传键值对 (可选)                                                     |
| `created_at` | `int64`（Unix 毫秒） | 发送入库时间戳                                                            |
| `expires_at` | `int64`（Unix 毫秒） | 消息过期时间 (默认 `created_at + 7天`)                                    |

---

### 2.5 `meta` (系统全局配置与元数据桶)

- **Bucket 标识**：`meta`
- **存储模式**：单字段单值（Key-Value 扁平存储），所有配置项以统一前缀组织。
- **Bark Relay 配置项**：

| Key                  | Value 示例              | 说明                                                                  |
| :------------------- | :---------------------- | :-------------------------------------------------------------------- |
| `bark_relay_enabled` | `"true"` / `"false"`    | 是否开启 Bark Relay 透明反向代理及双推转发功能                        |
| `bark_server_url`    | `"https://api.day.app"` | 上游目标 Bark-Server 地址 (支持自建服务)                              |
| `bark_route_path`    | `"/bark-relay"`         | 本地监听反向代理的路由前缀路径 (在 iOS Bark App 中作为服务器地址填入) |

---

## 3. 二级索引桶设计 (离线重放与范围检索)

为支持 macOS 客户端网络离线重连、合盖休眠唤醒后凭 `Last-Event-ID` 无损补发历史通知，系统维护了前缀范围扫描索引：

### 3.1 `idx_device_messages` (设备专属消息索引)

- **Key 构造**：`{device_id}:{Itob(message_id)}`
  - 前缀为 `{device_id}:`，后 8 字节为大端序 `uint64` 消息 ID。
- **Value**：8 字节大端序 `Itob(message_id)`。
- **检索方式**：当设备重连携带 `Last-Event-ID: afterID` 时，Cursor 寻址到 `{device_id}:{Itob(afterID + 1)}` 顺序正向扫描，在 $O(\log N + K)$ 复杂度内精准拉取该设备在离线期间错过的消息。

### 3.2 `idx_key_messages` (密钥消息流索引)

- **Key 构造**：`{key_id}:{Itob(message_id)}`
- **Value**：8 字节大端序 `Itob(message_id)`。
- **检索方式**：用于按发信密钥维度审计、统计或回溯历史消息。

### 3.3 `idx_group_messages` (分组消息索引)

- **Key 构造**：`{group}:{Itob(message_id)}`
- **Value**：8 字节大端序 `Itob(message_id)`。
- **检索方式**：支持按分组（如 `deploy`）高效拉取聚合通知。

---

## 4. 数据一致性与自动垃圾回收 (GC)

### 4.1 写入与更新事务

- **设备绑定**：普通 Secret Key 注册设备时，直接将其 `ID` 记录在 `devices` 记录的 `key_id` 中；后续上报若未传 `key_id` 则保持原有归属，严格保障设备隔离。
- **密钥事务双写**：`secret_keys` 桶与 `sk_hashes` 桶在同一个 bbolt Update 读写事务中完成增删改，确保哈希索引与数据实体绝对一致。

### 4.2 自动过期清理 (GC)

- **过期策略**：消息入库时默认设置 `expires_at = now + 7天`。
- **定时任务**：服务端内置后台定时协程（每小时触发一次）：
  - 游标扫描 `messages` 桶，检查若 `now > expires_at`：
  - 原子级联从 `idx_device_messages`、`idx_key_messages`、`idx_group_messages` 与主 `messages` 桶中删除物理记录；
  - 防止 bbolt 单文件由于长期积累消息产生无效膨胀，保障读写性能。
