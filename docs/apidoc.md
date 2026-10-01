# Tink REST API Reference

[English](apidoc.md) | [简体中文](apidoc.zh-CN.md)

> **Version**: v0.9.30 · **Base URL**: `http://<host>:<port>` (default `http://localhost:5021`)

Conventions: all bodies are UTF-8 JSON; boolean flags on keys/devices are `0 / 1`; timestamps are 13-digit Unix milliseconds; realtime delivery uses SSE (Server-Sent Events). Error `message` strings are localized via `Accept-Language` (any `zh*` tag returns Chinese, otherwise English).

---

## 1. Authentication & Limits

Three credential channels are checked in this order:

1. **Unix domain socket (implicit admin)** — requests arriving on `<data>/tink.sock` bypass auth and rate limits and act as an admin key. Used by the `tink-server token/device` CLI when the server is running.
2. **Bearer Secret Key** — `Authorization: Bearer sk-tink-…`. The raw key is SHA-256 hashed and looked up; disabled keys are rejected.
3. **Browser session cookie** — `tink_session` (HttpOnly, SameSite=Lax, Secure under TLS) issued by `POST /api/v1/login`; idle TTL 8 h, absolute TTL 24 h. Non-GET requests must additionally send `X-Tink-CSRF: 1`, else `403`.

Rate limits:

| Limiter | Budget | Response |
|---|---|---|
| Per credential (per-IP when unauthenticated) | 120 req/min | `429 rate limit exceeded` |
| Auth failures per IP | 20/min | `429 too many failed attempts, retry later` |
| SSE `GET /api/v1/events` | exempt from the 120/min limiter | — |

## 2. Response Envelope

All REST endpoints except `/api/v1/ping` and SSE handshake errors:

```json
{ "code": 0, "message": "ok", "data": { } }
```

On error, `code` equals the HTTP status and `data` is `null`. Any `OPTIONS` request returns bare `204`. Security headers (`CSP`, `X-Frame-Options: DENY`, `nosniff`) are set on every response; there are no CORS headers.

## 3. Endpoints

### 3.1 `GET /api/v1/ping` — health check (no auth)

Flat JSON, no envelope:

```json
{ "message": "pong", "version": "0.9.30", "build": "a524ca3", "st": 1761900000000 }
```

### 3.2 Session

| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/api/v1/login` | none | Body `{"token":"<secret key>"}` → sets `tink_session` cookie. `data`: `{"id","name","role","is_master","enabled","created_at"}` |
| POST | `/api/v1/logout` | any | Revokes the current session, clears the cookie. `data`: `{"logout":true}` |
| GET | `/api/v1/me` | any | Current key identity: `{"id","name","role","is_master","enabled","created_at"}` |

`/api/v1/login` returns `404` when the dashboard is disabled (`TINK_ENABLE_DASHBOARD=false`), except over UDS.

### 3.3 Devices

| Method | Path | Auth |
|---|---|---|
| POST | `/api/v1/devices` | any |
| GET | `/api/v1/devices` | any (admin sees all; user keys see only their own) |
| DELETE | `/api/v1/devices/{id}` | owner key or admin |

Register/upsert body: `{"id": "<device uuid>", "name": "<label>"}` (both required). The calling key is bound as `key_id`; re-registering preserves the existing binding and timestamps.

Device object:

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

`status` is runtime-only (`1` = SSE connected), never persisted. Deleting a device force-closes its SSE connection and returns `{"id","deleted":1}`.

### 3.4 `POST /api/v1/messages` — push (any auth)

| Field | Type | Required | Description |
|---|---|---|---|
| `title` / `body` | string | one of them | Notification text |
| `devices` | []string | see note | Target Tink device IDs |
| `bark_devices` | []string | see note | Target Bark device keys |
| `bark_params` | object | no | Extra keys merged into the upstream Bark `/push` payload (`level`, `icon`, `badge`, `copy`, `autoCopy`, …); `device_key` cannot be overridden |
| `url` | string | no | Opened when the notification is clicked |
| `sound` | string | no | e.g. `default` |
| `group` | string | no | Carried in the message and indexed (no group-query API yet) |
| `payload` | object | no | Arbitrary JSON attached to the message |

Note: at least one of `devices` / `bark_devices` is required — broadcast-without-target is disabled. Every listed Tink device must exist and (for user keys) belong to the caller. Bark forwarding requires **Bark Relay** enabled in settings with a non-empty upstream URL; otherwise each Bark target appends an entry to `bark_errors` (push to Tink still succeeds).

**Notification icon (macOS).** The macOS client cannot display a custom notification icon: the system always draws the app bundle icon, and no public API overrides it per message. Measured on 2026-10-01 — a `UNNotificationAttachment` built from a downloaded PNG is accepted by the API (`attachments.count == 1`, `add()` returns no error) yet renders nothing in the banner or Notification Center. `bark_params.icon` affects the iOS Bark app only (Apple's communication-notification avatar, which requires a Developer ID entitlement Tink does not ship), so putting an `icon` key in `payload` changes nothing on macOS today.

Success returns `201`:

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

`id` / `created_at` are present only when Tink devices were targeted; `bark_errors` only when some Bark push failed. Message IDs are monotonic `uint64`; messages expire after 7 days.

### 3.5 `GET /api/v1/events` — SSE stream (any auth)

Device identity via `X-Device-ID` header **or** `?device_id=` query param; the device must belong to the caller's key (admin exempt).

Offline replay: send `Last-Event-ID` header (or `?last_event_id=`) with the last seen numeric ID; the server replays up to **200** unexpired messages with `id > last_event_id` before switching to live delivery.

Frames:

```text
id: 42
event: notification
data: {"id":42,"key_id":"sk_…","devices":["…"],"title":"…","body":"…","url":"…","sound":"…","group":"…","payload":{…},"created_at":…,"expires_at":…}

event: ping
data: {}
```

A `ping` heartbeat is sent every 30 s; a failed heartbeat closes the connection — reconnect with the same device ID (a new connection replaces the stale one). Handshake errors are **raw JSON, not the envelope**: `400 {"error":"X-Device-ID header or device_id query required"}`, `401 {"error":"device not registered"}`, `403 {"error":"unauthorized device"}`.

### 3.6 Keys (admin only)

`/api/v1/tokens` is an alias for `/api/v1/keys`. All four routes return `404` when the dashboard is disabled (UDS unaffected).

| Method | Path | Body / Result |
|---|---|---|
| GET | `/api/v1/keys` | Array of `{"id","name","role","is_master","enabled","created_at","last_used"}` |
| POST | `/api/v1/keys` | `{"name":"ci-key","role":"user"}` (both optional; role ≠ `admin` → `user`) → `201` with `{"token","secret_key",…}` — the raw key is returned **once** |
| PUT | `/api/v1/keys/{id}` | Any of `{"name","enabled":0|1,"role"}`; an admin key cannot be disabled |
| DELETE | `/api/v1/keys/{id}` | Guards: cannot delete the key making the request, cannot delete admin keys |

Key format: `sk-tink-` + 64 random alphanumeric chars; ID = `sk_` + last 8 chars.

### 3.7 Settings (admin only)

`GET /api/v1/settings` → `{"bark_relay_enabled": false, "bark_server_url": "https://api.day.app", "bark_route_path": "/bark-relay"}`

`PUT /api/v1/settings` accepts the same keys (all optional, partial merge; real JSON booleans here). Values are normalized: `bark_server_url` trims trailing `/` (empty → `https://api.day.app`); `bark_route_path` gets a leading `/` (empty or `/` → `/bark-relay`).

### 3.8 Bark relay proxy

When `bark_relay_enabled`, every request to `{bark_route_path}/…` (e.g. `/bark-relay/push`, `/bark-relay/register`) is transparently reverse-proxied to `{bark_server_url}/…` with the prefix stripped — no auth required. Point the iOS Bark App at `http://<host>:5021/bark-relay` for full native proxying (registration, `device_key` issuing, pushes). Disabled → `404`; invalid upstream → `502`.

### 3.9 Web dashboard

The embedded SPA is served at `/dashboard` by default. `--dashboard-route` / `TINK_DASHBOARD_ROUTE` moves it (e.g. `/my-panel`); once customized, `/` returns `404` to avoid leaking the path. `TINK_ENABLE_DASHBOARD=false` removes all web surfaces and the login/keys API over TCP.

## 4. Error Message Catalogue

Messages are localized; English is the API contract. `%s` / `%v` are filled with details.

| HTTP | English | 中文 |
|---|---|---|
| 400 | `invalid json` | JSON 格式错误 |
| 400 | `id and name are required` | id 和 name 为必填项 |
| 400 | `missing device id` / `missing key id` | 缺少设备 ID / 缺少密钥 ID |
| 400 | `title or body is required` | title 与 body 至少填写一项 |
| 400 | `at least one target is required: specify 'devices' for Tink clients or 'bark_devices' for Bark clients` | 请至少指定一个目标… |
| 400 | `admin key cannot be disabled` / `admin key cannot be deleted` | 不能禁用/删除管理员密钥 |
| 400 | `cannot delete currently active secret key` | 不能删除当前正在使用的密钥 |
| 401 | `missing authorization header` | 缺少认证信息 (Authorization 头) |
| 401 | `invalid or expired secret key` | Secret Key 无效或已过期 |
| 403 | `csrf check failed` | CSRF 校验失败，请刷新页面后重试 |
| 403 | `forbidden: admin role required` | 禁止访问：需要管理员权限 |
| 403 | `forbidden: cannot delete device registered with another secret key` | 禁止访问：该设备注册于其他 Secret Key… |
| 403 | `device %s does not belong to your secret key` | 设备 %s 不属于当前 Secret Key |
| 404 | `endpoint not found` / `device not found` / `secret key not found` | 接口不存在 / 设备不存在 / Secret Key 不存在 |
| 405 | `method not allowed` | 请求方法不被允许 |
| 429 | `rate limit exceeded` / `too many failed attempts, retry later` | 请求过于频繁… / 失败次数过多… |
| 500 | `failed to generate message id` / `persist error: %s` / `generate secret key failed` | 生成消息 ID 失败 / 写入存储失败：%s / 生成 Secret Key 失败 |
| 502 | `invalid upstream bark server url` / `bark relay proxy error: %v` | Bark 上游服务地址无效 / Bark 转发失败：%v |

## 5. Environment & CLI

| Variable | Flag | Default | Purpose |
|---|---|---|---|
| `TINK_PORT` | `serve -p, --port` | `5021` | Listen port |
| `TINK_DATA_DIR` | `-d, --data` | `./data` | Data directory (`tink.db`, `tink.sock`) |
| `TINK_INITIAL_TOKEN` | `serve --token` | random | Initial admin key; applied **only** when the DB has zero keys |
| `TINK_ENABLE_DASHBOARD` | `serve --enable-dashboard` | `true` | `false`/`0` disables web surfaces + login/keys API |
| `TINK_DASHBOARD_ROUTE` | `serve --dashboard-route` | `/dashboard` | Custom console path |

CLI (`tink-server`): `serve`, `key` (alias `token`) `list|create -n -r|enable|disable|delete`, `device list|delete`, `version`. Management subcommands talk to the running server over UDS automatically, falling back to direct (offline) bbolt access when the server is stopped.

## 6. Related Docs

- Database schema: [database_schema.md](database_schema.md) / [中文](database_schema.zh-CN.md)
- Project overview: [README.md](../README.md) / [中文](../README.zh-CN.md)
