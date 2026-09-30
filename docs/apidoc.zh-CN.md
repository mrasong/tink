# Tink REST API 接口文档

[English](apidoc.md) | [简体中文](apidoc.zh-CN.md)

> **版本**：v0.9.30 · **Base URL**：`http://<host>:<port>`（默认 `http://localhost:5021`）

协议规范：全部接口采用 UTF-8 编码 JSON 格式交互；密钥/设备的布尔标识统一使用 `0 / 1`；业务时间字段统一使用 13 位 Unix 毫秒时间戳；实时长连接通过 SSE (Server-Sent Events) 传输。错误 `message` 按 `Accept-Language` 本地化（含 `zh` 语言标签返回中文，其余返回英文）。

---

## 1. 鉴权与限流

按以下优先级识别三种凭据通道：

1. **Unix 域套接字（隐式管理员）** — 经 `<data>/tink.sock` 进入的请求免鉴权、免限流，等同管理员密钥。服务端运行时的 `tink-server token/device` CLI 即走此通道。
2. **Bearer Secret Key** — `Authorization: Bearer sk-tink-…`。原始密钥经 SHA-256 哈希后查表；已禁用的密钥直接拒绝。
3. **浏览器会话 Cookie** — 由 `POST /api/v1/login` 下发的 `tink_session`（HttpOnly、SameSite=Lax、TLS 下 Secure）；空闲 8 小时、最长 24 小时过期。非 GET 请求必须额外携带 `X-Tink-CSRF: 1`，否则 `403`。

限流策略：

| 限流器 | 额度 | 响应 |
|---|---|---|
| 按凭据计（未认证时按 IP） | 120 次/分钟 | `429 rate limit exceeded` |
| 按 IP 计的认证失败次数 | 20 次/分钟 | `429 too many failed attempts, retry later` |
| SSE `GET /api/v1/events` | 不受 120 次/分钟限制 | — |

## 2. 统一响应信封

除 `/api/v1/ping` 与 SSE 握手错误外，所有 REST 接口统一返回：

```json
{ "code": 0, "message": "ok", "data": { } }
```

出错时 `code` 等于 HTTP 状态码，`data` 为 `null`。任意 `OPTIONS` 请求直接返回裸 `204`。所有响应携带安全头（`CSP`、`X-Frame-Options: DENY`、`nosniff`）；服务端不下发 CORS 头。

## 3. 接口明细

### 3.1 `GET /api/v1/ping` — 健康检查（免鉴权）

平铺 JSON，无信封：

```json
{ "message": "pong", "version": "0.9.30", "build": "a524ca3", "st": 1761900000000 }
```

### 3.2 会话

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| POST | `/api/v1/login` | 无 | 请求体 `{"token":"<secret key>"}` → 下发 `tink_session` Cookie。`data`：`{"id","name","role","is_master","enabled","created_at"}` |
| POST | `/api/v1/logout` | 任意 | 吊销当前会话并清除 Cookie。`data`：`{"logout":true}` |
| GET | `/api/v1/me` | 任意 | 当前密钥身份：`{"id","name","role","is_master","enabled","created_at"}` |

控制台被禁用（`TINK_ENABLE_DASHBOARD=false`）时 `/api/v1/login` 返回 `404`（UDS 通道除外）。

### 3.3 设备

| 方法 | 路径 | 鉴权 |
|---|---|---|
| POST | `/api/v1/devices` | 任意 |
| GET | `/api/v1/devices` | 任意（管理员见全部；普通密钥只见自己的） |
| DELETE | `/api/v1/devices/{id}` | 归属密钥或管理员 |

注册（upsert）请求体：`{"id": "<设备 UUID>", "name": "<展示名>"}`（均必填）。调用方密钥自动绑定为 `key_id`；重复注册保留原有绑定与时间戳。

设备对象：

```json
{
  "id": "9F2C…",
  "key_id": "sk_3WHyWNzJ",
  "name": "MacBook Pro",
  "status": 1,
  "created_at": 1761900000000,
  "last_connected_at": 1761900000000,
  "last_disconnected_at": 0
}
```

`status` 为运行时状态（`1` = SSE 在线），不落库。删除设备会强制关闭其 SSE 连接，返回 `{"id","deleted":1}`。

### 3.4 `POST /api/v1/messages` — 推送（任意鉴权）

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `title` / `body` | string | 二选一 | 通知文本 |
| `devices` | []string | 见注 | Tink 设备 ID 列表 |
| `bark_devices` | []string | 见注 | Bark 设备 key 列表 |
| `bark_params` | object | 否 | 合并进上游 Bark `/push` 的额外键值（`level`、`icon`、`badge`、`copy`、`autoCopy` 等）；`device_key` 不可覆盖 |
| `url` | string | 否 | 点击通知时打开 |
| `sound` | string | 否 | 如 `default` |
| `group` | string | 否 | 随消息携带并建索引（暂无分组查询接口） |
| `payload` | object | 否 | 附加的任意 JSON |

注：`devices` 与 `bark_devices` 至少指定其一——无目标广播已被禁用。所列 Tink 设备必须存在，且普通密钥只能投递给自己名下的设备。Bark 转发要求系统设置中开启 **Bark Relay** 且上游地址非空，否则每个 Bark 目标会向 `bark_errors` 追加一条错误（Tink 侧推送不受影响）。

成功返回 `201`：

```json
{
  "code": 0, "message": "ok",
  "data": {
    "dispatched_tink": 2,
    "dispatched_bark": 1,
    "id": 42,
    "created_at": 1761900000000,
    "bark_errors": ["bark relay is disabled or bark_server_url not configured"]
  }
}
```

`id` / `created_at` 仅在存在 Tink 目标时返回；`bark_errors` 仅在 Bark 失败时返回。消息 ID 为单调递增 `uint64`；消息 7 天后过期。

### 3.5 `GET /api/v1/events` — SSE 长连接（任意鉴权）

设备身份通过 `X-Device-ID` 头**或** `?device_id=` 查询参数传入；设备须属于调用方密钥（管理员豁免）。

离线重放：携带 `Last-Event-ID` 头（或 `?last_event_id=`）为最后收到的数字 ID，服务端先补发最多 **200** 条 `id > last_event_id` 的未过期消息，再转入实时推送。

帧格式：

```text
id: 42
event: notification
data: {"id":42,"key_id":"sk_…","devices":["…"],"title":"…","body":"…","url":"…","sound":"…","group":"…","payload":{…},"created_at":…,"expires_at":…}

event: ping
data: {}
```

每 30 秒发送一次 `ping` 心跳；心跳写失败即断开——请用同一设备 ID 重连（新连接会顶替旧连接）。握手错误为**裸 JSON、不走信封**：`400 {"error":"X-Device-ID header or device_id query required"}`、`401 {"error":"device not registered"}`、`403 {"error":"unauthorized device"}`。

### 3.6 密钥管理（仅管理员）

`/api/v1/tokens` 是 `/api/v1/keys` 的别名。控制台被禁用时以下四个路由均返回 `404`（UDS 不受影响）。

| 方法 | 路径 | 请求 / 结果 |
|---|---|---|
| GET | `/api/v1/keys` | 返回 `{"id","name","role","is_master","enabled","created_at","last_used"}` 数组 |
| POST | `/api/v1/keys` | `{"name":"ci-key","role":"user"}`（均可选；role ≠ `admin` 一律按 `user`）→ `201`，返回 `{"token","secret_key",…}` — 原始密钥**仅返回这一次** |
| PUT | `/api/v1/keys/{id}` | 可分别传 `{"name","enabled":0|1,"role"}`；管理员密钥不可被禁用 |
| DELETE | `/api/v1/keys/{id}` | 保护规则：不能删除本次请求所用密钥；不能删除管理员密钥 |

密钥格式：`sk-tink-` + 64 位随机字母数字；ID = `sk_` + 原始密钥末 8 位。

### 3.7 系统设置（仅管理员）

`GET /api/v1/settings` → `{"bark_relay_enabled": false, "bark_server_url": "https://api.day.app", "bark_route_path": "/bark-relay"}`

`PUT /api/v1/settings` 接受同名键（均可选、增量合并；此处布尔为真正的 JSON bool）。存储前规范化：`bark_server_url` 去尾部 `/`（置空回落 `https://api.day.app`）；`bark_route_path` 补前导 `/`（空或 `/` 回落 `/bark-relay`）。

### 3.8 Bark 透明代理

开启 `bark_relay_enabled` 后，发往 `{bark_route_path}/…`（如 `/bark-relay/push`、`/bark-relay/register`）的请求会被去前缀后透明反向代理至 `{bark_server_url}/…`——免鉴权。在 iOS Bark App 中把服务器地址填为 `http://<host>:5021/bark-relay` 即获得完整原生代理（注册、`device_key` 签发、推送）。未开启 → `404`；上游地址非法 → `502`。

### 3.9 Web 管理控制台

内嵌 SPA 默认挂载于 `/dashboard`。可用 `--dashboard-route` / `TINK_DASHBOARD_ROUTE` 自定义（如 `/my-panel`）；自定义后根路径 `/` 返回 `404`，避免泄露私有路径。`TINK_ENABLE_DASHBOARD=false` 会移除全部页面，并在 TCP 通道上禁用 login/keys 接口。

## 4. 错误文案对照表

错误消息已本地化；英文即接口契约，`%s` / `%v` 为动态详情。

| HTTP | 英文（契约） | 中文 |
|---|---|---|
| 400 | `invalid json` | JSON 格式错误 |
| 400 | `id and name are required` | id 和 name 为必填项 |
| 400 | `missing device id` / `missing key id` | 缺少设备 ID / 缺少密钥 ID |
| 400 | `title or body is required` | title 与 body 至少填写一项 |
| 400 | `at least one target is required: specify 'devices' for Tink clients or 'bark_devices' for Bark clients` | 请至少指定一个目标：Tink 设备填入 'devices'，Bark 设备填入 'bark_devices' |
| 400 | `admin key cannot be disabled` / `admin key cannot be deleted` | 不能禁用管理员密钥 / 不能删除管理员密钥 |
| 400 | `cannot delete currently active secret key` | 不能删除当前正在使用的密钥 |
| 401 | `missing authorization header` | 缺少认证信息 (Authorization 头) |
| 401 | `invalid or expired secret key` | Secret Key 无效或已过期 |
| 403 | `csrf check failed` | CSRF 校验失败，请刷新页面后重试 |
| 403 | `forbidden: admin role required` | 禁止访问：需要管理员权限 |
| 403 | `forbidden: cannot delete device registered with another secret key` | 禁止访问：该设备注册于其他 Secret Key，无法删除 |
| 403 | `device %s does not belong to your secret key` | 设备 %s 不属于当前 Secret Key |
| 404 | `endpoint not found` / `device not found` / `secret key not found` | 接口不存在 / 设备不存在 / Secret Key 不存在 |
| 405 | `method not allowed` | 请求方法不被允许 |
| 429 | `rate limit exceeded` / `too many failed attempts, retry later` | 请求过于频繁，请稍后再试 / 失败次数过多，请稍后再试 |
| 500 | `failed to generate message id` / `persist error: %s` / `generate secret key failed` | 生成消息 ID 失败 / 写入存储失败：%s / 生成 Secret Key 失败 |
| 502 | `invalid upstream bark server url` / `bark relay proxy error: %v` | Bark 上游服务地址无效 / Bark 转发失败：%v |

## 5. 环境变量与 CLI

| 变量 | 参数 | 默认 | 用途 |
|---|---|---|---|
| `TINK_PORT` | `serve -p, --port` | `5021` | 监听端口 |
| `TINK_DATA_DIR` | `-d, --data` | `./data` | 数据目录（`tink.db`、`tink.sock`） |
| `TINK_INITIAL_TOKEN` | `serve --token` | 随机 | 初始管理员密钥；**仅**在库内零密钥时生效 |
| `TINK_ENABLE_DASHBOARD` | `serve --enable-dashboard` | `true` | `false`/`0` 关闭页面及 login/keys 接口 |
| `TINK_DASHBOARD_ROUTE` | `serve --dashboard-route` | `/dashboard` | 自定义控制台路径 |

CLI（`tink-server`）：`serve`、`key`（别名 `token`）`list|create -n -r|enable|disable|delete`、`device list|delete`、`version`。管理类子命令在服务运行时自动走 UDS，服务停止时自动降级为离线直写 bbolt。

## 6. 相关文档

- 数据库结构：[database_schema.zh-CN.md](database_schema.zh-CN.md) / [English](database_schema.md)
- 项目总览：[README.zh-CN.md](../README.zh-CN.md) / [English](../README.md)
