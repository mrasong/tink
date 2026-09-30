# Tink 服务端 REST API 文档

> **版本**：v0.2.0  
> **Base URL**：`http://<server-host>:<port>` (默认 `http://localhost:5021`)
> **协议规范**：全部接口采用 UTF-8 编码 JSON 格式交互；布尔字段统一使用 `1 (启用/真)` / `0 (禁用/假)`；所有业务时间字段统一使用 13 位 Unix 毫秒时间戳；实时长连接通过 SSE (Server-Sent Events) 传输。

---

## 目录

- [1. 通用协议与格式规范](#1-通用协议与格式规范)
  - [1.1 鉴权机制 (Bearer Secret Key)](#11-鉴权机制-bearer-secret-key)
  - [1.2 统一响应结构 (Envelope)](#12-统一响应结构-envelope)
  - [1.3 全局状态码规范](#13-全局状态码规范)
  - [1.4 频率限制 (Rate Limiting)](#14-频率限制-rate-limiting)
- [2. 系统与健康检查接口](#2-系统与健康检查接口)
  - [2.1 心跳检测 (Ping)](#21-心跳检测-ping)
- [3. 身份与凭据查询](#3-身份与凭据查询)
  - [3.1 获取当前 Secret Key 身份信息](#31-获取当前-secret-key-身份信息)
- [4. 设备管理接口 (Devices)](#4-设备管理接口-devices)
  - [4.1 注册/更新设备](#41-注册更新设备)
  - [4.2 获取设备列表](#42-获取设备列表)
  - [4.3 删除指定设备](#43-删除指定设备)
- [5. 消息推送接口 (Messages)](#5-消息推送接口-messages)
  - [5.1 发送消息通知](#51-发送消息通知)
- [6. 实时通道接口 (SSE)](#6-实时通道接口-sse)
  - [6.1 订阅实时消息流](#61-订阅实时消息流)
- [7. 密钥管理接口 (Secret Keys, 仅 Admin 角色可用)](#7-密钥管理接口-secret-keys-仅-admin-角色可用)
  - [7.1 获取密钥列表](#71-获取密钥列表)
  - [7.2 创建新密钥](#72-创建新密钥)
  - [7.3 修改密钥 (重命名/启禁用/角色变更)](#73-修改密钥-重命名启禁用角色变更)
  - [7.4 删除/吊销密钥](#74-删除吊销密钥)
- [8. 附录：服务配置与环境变量](#8-附录服务配置与环境变量)

---

## 1. 通用协议与格式规范

### 1.1 鉴权机制 (Bearer Secret Key)

除公开的心跳检测端点 (`/api/v1/ping`) 外，所有 API 均需在 HTTP Header 中携带 Secret Key：

```http
Authorization: Bearer <your-secret-key>
```

- **Admin 角色** (`role: "admin"`)：系统管理员密钥，拥有全部最高权限（统管所有设备、创建/禁用/删除其它 Secret Key 等）。
- **User 角色** (`role: "user"`)：业务发信与设备绑定凭据，具备设备绑定和定向推送权限（受权限隔离保护，仅能查看与操作自身关联的设备，无权管理密钥）。

### 1.2 统一响应结构 (Envelope)

所有接口均统一返回标准化 JSON 信封封装，布尔字段统一使用 `1 / 0`：

```json
{
  "code": 0,
  "message": "ok",
  "data": { ... }
}
```

- `code`：状态业务码（`0` 表示操作成功，非 `0` 表示业务或请求错误，通常对应 HTTP 状态码如 `400`、`401`、`403`、`404`、`429`、`500` 等）。
- `message`：提示信息（成功通常为 `"ok"`，错误时为具体错误原因）。
- `data`：成功时携带的业务数据 payload；失败时通常为 `null`。

### 1.3 全局状态码规范

| HTTP 状态码                 | 业务 Code | 说明                                                                |
| :-------------------------- | :-------- | :------------------------------------------------------------------ |
| `200 OK`                    | `0`       | 请求成功                                                            |
| `201 Created`               | `0`       | 资源创建成功 (如发信、创建 Key)                                     |
| `400 Bad Request`           | `400`     | 参数错误、缺少必填字段（如未指定目标设备）或非法操作                |
| `401 Unauthorized`          | `401`     | 未提供 Secret Key、Key 无效或已被禁用 (`enabled: 0`)                |
| `403 Forbidden`             | `403`     | 权限不足（如 User 角色试图调用 Key 管理接口，或设备不归属当前 Key） |
| `404 Not Found`             | `404`     | 路由不存在或资源未找到                                              |
| `429 Too Many Requests`     | `429`     | 请求超过速率限制                                                    |
| `500 Internal Server Error` | `500`     | 服务端内部存储或执行错误                                            |

### 1.4 频率限制 (Rate Limiting)

- 默认限流策略：单个 IP 每分钟最多允许 120 次短请求。
- 超过限流阈值时响应 HTTP `429 Too Many Requests`。

---

## 2. 系统与健康检查接口

### 2.1 心跳检测 (Ping)

检测服务端运行状态、获取服务版本及当前服务端 Unix 毫秒时间戳。该接口为公开健康检测端点，无需鉴权，直接返回扁平 JSON 数据（不包装外层 `Envelope`）。

- **请求方式**：`GET /api/v1/ping`
- **请求头**：无需 `Authorization`

**响应示例**：

```json
{
  "message": "pong",
  "version": "0.2.0",
  "st": 1789141200000
}
```

---

## 3. 身份与凭据查询

### 3.1 获取当前 Secret Key 身份信息

查询当前发起调用的 Secret Key 详情与权限角色（前端控制台用于区分 Admin 与 User 界面权限）。

- **请求方式**：`GET /api/v1/me`
- **请求头**：`Authorization: Bearer <Secret-Key>`

**响应示例**：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "id": "sk_3WHyWNzJ",
    "name": "GitHub CI Key",
    "role": "user",
    "enabled": 1,
    "created_at": 1789258500000
  }
}
```

---

## 4. 设备管理接口 (Devices)

### 4.1 注册/更新设备

macOS 客户端上线或初次启动时调用，将本机设备信息登记至服务端，并与当前 Secret Key 建立归属关联。

- **请求方式**：`POST /api/v1/devices`
- **请求头**：
  - `Authorization: Bearer <Secret-Key>`
  - `Content-Type: application/json`

**请求参数 (Body)**：

| 字段名 | 类型     | 必填 | 描述                              | 示例                |
| :----- | :------- | :--- | :-------------------------------- | :------------------ |
| `id`   | `string` | 是   | 客户端持久化设备唯一识别码 (UUID) | `"dev-macbook-pro"` |
| `name` | `string` | 是   | 设备名称 (供控制台识别)           | `"My MacBook Pro"`  |

**请求体示例**：

```json
{
  "id": "dev-macbook-pro",
  "name": "My MacBook Pro"
}
```

**响应示例**：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "id": "dev-macbook-pro",
    "key_id": "sk_3WHyWNzJ",
    "name": "My MacBook Pro",
    "created_at": 1789258530000,
    "last_connected_at": 1789258530000,
    "last_disconnected_at": 0
  }
}
```

---

### 4.2 获取设备列表

查询已登记的设备列表与在线状态。

- **Admin 角色**：返回系统所有注册的设备；
- **User 角色**：仅返回绑定在该 Secret Key 下的设备（隔离保护）。

- **请求方式**：`GET /api/v1/devices`
- **请求头**：`Authorization: Bearer <Secret-Key>`

**响应示例**：

```json
{
  "code": 0,
  "message": "ok",
  "data": [
    {
      "id": "dev-macbook-pro",
      "key_id": "sk_3WHyWNzJ",
      "name": "My MacBook Pro",
      "status": 1,
      "created_at": 1789258530000,
      "last_connected_at": 1789258530000,
      "last_disconnected_at": 1789258800000
    }
  ]
}
```

---

### 4.3 删除指定设备

注销并删除设备。若该设备当前处于 SSE 在线连接状态，服务端会自动断开连接。

- **权限限制**：Admin 可删除任意设备；User 角色仅能删除归属于自身 Secret Key 的设备。

- **请求方式**：`DELETE /api/v1/devices/{id}`
- **请求头**：`Authorization: Bearer <Secret-Key>`

**响应示例**：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "id": "dev-macbook-pro",
    "deleted": 1
  }
}
```

---

## 5. 消息推送接口 (Messages)

### 5.1 发送消息通知

向已登记的目标 Mac 设备即时投递通知。系统已支持多设备，**必须显式指定接收的目标设备列表（已彻底禁用无目标广播）**。

- **请求方式**：`POST /api/v1/messages`
- **请求头**：
  - `Authorization: Bearer <Secret-Key>`
  - `Content-Type: application/json`

**请求参数 (Body)**：

| 字段名    | 类型       | 必填     | 描述                                                                      | 示例                                     |
| :-------- | :--------- | :------- | :------------------------------------------------------------------------ | :--------------------------------------- |
| `devices` | `string[]` | **是**   | 接收通知的目标设备 ID 列表（不能为空；User 角色只能向属于自己的设备发信） | `["dev-macbook-pro"]`                    |
| `title`   | `string`   | 条件必填 | 通知标题（title 与 body 至少填一个）                                      | `"构建成功通知"`                         |
| `body`    | `string`   | 条件必填 | 通知主体内容                                                              | `"CI Pipeline #1024 构建成功，耗时 45s"` |
| `url`     | `string`   | 否       | 点击通知后自动在浏览器唤起的跳转链接                                      | `"https://ci.example.com/build/1024"`    |
| `sound`   | `string`   | 否       | 提示音名称（如 `default`、`glass`、`bell`、`silent`）                     | `"default"`                              |
| `group`   | `string`   | 否       | 通知分组标识（客户端据此归类聚合）                                        | `"deploy"`                               |
| `payload` | `object`   | 否       | 自定义键值对扩展数据                                                      | `{"env": "production"}`                  |

**请求体示例**：

```json
{
  "devices": ["dev-macbook-pro"],
  "title": "生产环境部署完成",
  "body": "v1.2.0 已上线并在集群中完成健康检查",
  "url": "https://dashboard.example.com",
  "sound": "default",
  "group": "deploy",
  "payload": {
    "build_id": "20260913-01"
  }
}
```

**响应示例**：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "id": 1,
    "dispatched": 1,
    "created_at": 1789258800000
  }
}
```

---

## 6. 实时通道接口 (SSE)

### 6.1 订阅实时消息流

macOS 客户端或其它订阅端通过 HTTP 长连接接收服务器推送的实时通知事件，支持断网自动补发重放。

- **请求方式**：`GET /api/v1/events`
- **请求头**：
  - `Authorization: Bearer <Secret-Key>`
  - `Accept: text/event-stream`
  - `X-Device-ID: <device-uuid>`（当前订阅设备的 UUID，用于定向分发与保活）
  - `Last-Event-ID: <uint64>`（可选：上一次成功接收的消息 ID，重连时服务端自动重放遗漏的历史离线消息）

**服务端返回流格式**：

```text
event: ping
data: {}

id: 1
data: {"id":1,"key_id":"sk_3WHyWNzJ","devices":["dev-macbook-pro"],"group":"deploy","title":"生产环境部署完成","body":"v1.2.0 已上线","url":"https://dashboard.example.com","sound":"default","created_at":1789258800000}
```

- 心跳保活：服务端每隔 30 秒发送一次 `event: ping`，防止中间代理超时断开。

---

## 7. 密钥管理接口 (Secret Keys, 仅 Admin 角色可用)

> **权限说明**：本组所有接口均被 `RequireAdminRole` 拦截，仅允许持有 **Admin 角色** 的 Secret Key 发起请求。普通 User 角色访问均返回 `403 Forbidden`。

### 7.1 获取密钥列表

获取系统所有创建的 Secret Key 清单。

- **请求方式**：`GET /api/v1/keys` (兼容别名 `GET /api/v1/tokens`)
- **请求头**：`Authorization: Bearer <Admin-Secret-Key>`

**响应示例**：

```json
{
  "code": 0,
  "message": "ok",
  "data": [
    {
      "id": "admin",
      "name": "Admin Secret Key",
      "role": "admin",
      "enabled": 1,
      "created_at": 1789257600000,
      "last_used": 1789258800000
    },
    {
      "id": "sk_3WHyWNzJ",
      "name": "GitHub Actions CI",
      "role": "user",
      "enabled": 1,
      "created_at": 1789258200000,
      "last_used": 1789258680000
    }
  ]
}
```

---

### 7.2 创建新密钥

生成新的 Secret Key，生成的明文密钥在响应中**仅返回一次**，服务端只持久化单向 SHA-256 Hash。

- **请求方式**：`POST /api/v1/keys` (兼容别名 `POST /api/v1/tokens`)
- **请求头**：
  - `Authorization: Bearer <Admin-Secret-Key>`
  - `Content-Type: application/json`

**请求参数 (Body)**：

| 字段名 | 类型     | 必填 | 描述                                            | 示例                 |
| :----- | :------- | :--- | :---------------------------------------------- | :------------------- |
| `name` | `string` | 否   | 密钥用途说明备注（默认 `"default"`）            | `"GitHub CI Runner"` |
| `role` | `string` | 否   | 密钥角色：`"user"` 或 `"admin"` (默认 `"user"`) | `"user"`             |

**请求体示例**：

```json
{
  "name": "GitHub CI Runner",
  "role": "user"
}
```

**响应示例**：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "secret_key": "sk-tink-d2b3c4e5f6a7b8c9d0e1f2a3b4c5d6e7",
    "id": "sk_b4c5d6e7",
    "name": "GitHub CI Runner",
    "role": "user",
    "enabled": 1,
    "created_at": 1789259100000
  }
}
```

---

### 7.3 修改密钥 (重命名/启禁用/角色变更)

更新密钥名称、启/禁用状态（`1 / 0`）或角色。已被禁用的密钥（`enabled: 0`）无法通过鉴权。

- **请求方式**：`PUT /api/v1/keys/{id}` (兼容别名 `PUT /api/v1/tokens/{id}`)
- **请求头**：
  - `Authorization: Bearer <Admin-Secret-Key>`
  - `Content-Type: application/json`

**请求参数 (Body)**：

| 字段名    | 类型     | 必填 | 描述                                                    | 示例                    |
| :-------- | :------- | :--- | :------------------------------------------------------ | :---------------------- |
| `name`    | `string` | 否   | 新密钥名称                                              | `"Production Deployer"` |
| `enabled` | `uint8`  | 否   | 是否启用：`1` 为启用，`0` 为禁用 (Admin 密钥禁止被禁用) | `0`                     |
| `role`    | `string` | 否   | 变更角色 (`"user"` 或 `"admin"`)                        | `"user"`                |

**请求体示例**：

```json
{
  "name": "Production Deployer",
  "enabled": 0
}
```

**响应示例**：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "id": "sk_b4c5d6e7",
    "name": "Production Deployer",
    "role": "user",
    "enabled": 0,
    "last_used": 1789259100000
  }
}
```

---

### 7.4 删除/吊销密钥

永久注销并删除指定的 Secret Key。

- **安全保护机制**：
  - **Admin 保护**：禁止删除 Admin 密钥；
  - **防自删保护**：禁止删除当前发起调用的密钥自身。

- **请求方式**：`DELETE /api/v1/keys/{id}` (兼容别名 `DELETE /api/v1/tokens/{id}`)
- **请求头**：`Authorization: Bearer <Admin-Secret-Key>`

**响应示例**：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "id": "sk_b4c5d6e7",
    "deleted": 1
  }
}
```

---

## 8. 附录：服务配置与环境变量

| 环境变量                | CLI 参数             | 默认值       | 说明                                                                    |
| :---------------------- | :------------------- | :----------- | :---------------------------------------------------------------------- |
| `TINK_PORT`             | `-p, --port`         | `5021`       | 服务端监听 HTTP 端口                                                    |
| `TINK_DATA_DIR`         | `-d, --data`         | `./data`     | bbolt 嵌入式数据库存储目录                                              |
| `TINK_INITIAL_TOKEN`    | `--token`            | 自动生成     | 服务首次启动时的初始 Admin Secret Key                                   |
| `TINK_ENABLE_DASHBOARD` | `--enable-dashboard` | `true`       | 是否启用内置 Web 控制台及 Key 管理接口                                  |
| `TINK_DASHBOARD_ROUTE`  | `--dashboard-route`  | `/dashboard` | 自定义 Web 控制台访问路径（配置自定义后根路径 `/` 自动返回 404 防探测） |
