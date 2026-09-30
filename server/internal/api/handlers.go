package api

import (
	"encoding/json"
	"github.com/mrasong/tink/server/internal/i18n"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mrasong/tink/server/internal/auth"
	"github.com/mrasong/tink/server/internal/sse"
	"github.com/mrasong/tink/server/internal/store"
)

type Handler struct {
	store *store.Store
	hub   *sse.Hub
}

type deviceResponse struct {
	ID                 string `json:"id"`
	KeyID              string `json:"key_id,omitempty"`
	Name               string `json:"name"`
	Status             uint8  `json:"status"`
	CreatedAt          int64  `json:"created_at"`
	LastConnectedAt    int64  `json:"last_connected_at"`
	LastDisconnectedAt int64  `json:"last_disconnected_at"`
}

func makeDeviceResponse(device *store.Device, status uint8) deviceResponse {
	return deviceResponse{
		ID: device.ID, KeyID: device.KeyID, Name: device.Name, Status: status,
		CreatedAt: device.CreatedAt, LastConnectedAt: device.LastConnectedAt,
		LastDisconnectedAt: device.LastDisconnectedAt,
	}
}

func NewHandler(s *store.Store, h *sse.Hub) *Handler {
	return &Handler{
		store: s,
		hub:   h,
	}
}

// 当前服务端版本号常量
var Version = "dev"
var Build = ""

// Ping GET /api/v1/ping
// 响应服务端心跳状态、版本号及当前服务端 Unix 时间戳 (直接返回扁平 JSON，不包 Envelope)
func (h *Handler) Ping(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"message": "pong",
		"version": Version,
		"build":   Build,
		"st":      time.Now().UnixMilli(),
	})
}

// HealthCheck 保持向后兼容的方法签名别名
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.Ping(w, r)
}

// GetMe GET /api/v1/me
// 返回当前登录 Secret Key 的身份信息及权限角色（用于前端 Dashboard 角色判定）
func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
	sk := r.Context().Value(ContextKeySecretKey).(*store.SecretKey)

	JSONSuccess(w, http.StatusOK, map[string]any{
		"id":         sk.ID,
		"name":       sk.Name,
		"role":       sk.Role,
		"is_master":  sk.IsAdmin(), // 保留兼容字段
		"enabled":    sk.Enabled,
		"created_at": sk.CreatedAt,
	})
}

// RegisterDevice POST /api/v1/devices
func (h *Handler) RegisterDevice(w http.ResponseWriter, r *http.Request) {
	sk := r.Context().Value(ContextKeySecretKey).(*store.SecretKey)

	var req struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, r, http.StatusBadRequest, i18n.MsgInvalidJSON)
		return
	}
	if req.ID == "" || req.Name == "" {
		JSONError(w, r, http.StatusBadRequest, i18n.MsgDeviceFieldsRequired)
		return
	}

	keyID := ""
	if sk != nil {
		keyID = sk.ID
	}

	dev := &store.Device{
		ID:    req.ID,
		KeyID: keyID,
		Name:  req.Name,
	}

	if err := h.store.UpsertDevice(dev); err != nil {
		JSONError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	status := uint8(0)
	if h.hub != nil && h.hub.IsDeviceConnected(dev.ID) {
		status = 1
	}
	JSONSuccess(w, http.StatusOK, makeDeviceResponse(dev, status))
}

// ListDevices GET /api/v1/devices
func (h *Handler) ListDevices(w http.ResponseWriter, r *http.Request) {
	sk := r.Context().Value(ContextKeySecretKey).(*store.SecretKey)

	var devices []*store.Device
	var err error

	// 权限分流：Admin 查看全部设备；普通 SK 仅查看自身绑定的设备
	if sk != nil && !sk.IsAdmin() {
		devices, err = h.store.ListDevicesByKey(sk.ID)
	} else {
		devices, err = h.store.ListDevices()
	}

	if err != nil {
		JSONError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	// 动态注入 Hub 实时在线/离线状态
	if devices == nil {
		devices = make([]*store.Device, 0)
	}

	responses := make([]deviceResponse, 0, len(devices))
	for _, dev := range devices {
		status := uint8(0)
		if h.hub != nil && h.hub.IsDeviceConnected(dev.ID) {
			status = 1
		}
		responses = append(responses, makeDeviceResponse(dev, status))
	}

	JSONSuccess(w, http.StatusOK, responses)
}

// DeleteDevice DELETE /api/v1/devices/:id
func (h *Handler) DeleteDevice(w http.ResponseWriter, r *http.Request) {
	sk := r.Context().Value(ContextKeySecretKey).(*store.SecretKey)

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		JSONError(w, r, http.StatusBadRequest, i18n.MsgMissingDeviceID)
		return
	}
	devID := parts[3]

	dev, err := h.store.GetDevice(devID)
	if err != nil {
		JSONError(w, r, http.StatusNotFound, i18n.MsgDeviceNotFound)
		return
	}

	// 普通 SK 只能删除自己关联绑定的设备
	if sk != nil && !sk.IsAdmin() && dev.KeyID != sk.ID {
		JSONError(w, r, http.StatusForbidden, i18n.MsgDeviceDeleteForbidden)
		return
	}

	// 从数据库中删除并踢出 SSE Hub 连接
	if err := h.store.DeleteDevice(devID); err != nil {
		JSONError(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	h.hub.Unregister(devID)

	JSONSuccess(w, http.StatusOK, map[string]any{"id": devID, "deleted": 1})
}

// SendMessage POST /api/v1/messages
func (h *Handler) SendMessage(w http.ResponseWriter, r *http.Request) {
	sk := r.Context().Value(ContextKeySecretKey).(*store.SecretKey)

	var req struct {
		Devices     []string       `json:"devices"`
		BarkDevices []string       `json:"bark_devices"`
		BarkParams  map[string]any `json:"bark_params"`
		Group       string         `json:"group"`
		Title       string         `json:"title"`
		Body        string         `json:"body"`
		URL         string         `json:"url"`
		Sound       string         `json:"sound"`
		Payload     map[string]any `json:"payload"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, r, http.StatusBadRequest, i18n.MsgInvalidJSON)
		return
	}
	if req.Title == "" && req.Body == "" {
		JSONError(w, r, http.StatusBadRequest, i18n.MsgTitleOrBodyRequired)
		return
	}

	// 要求至少指定一个目标（Tink 设备 或 Bark 设备）
	if len(req.Devices) == 0 && len(req.BarkDevices) == 0 {
		JSONError(w, r, http.StatusBadRequest, i18n.MsgPushTargetRequired)
		return
	}

	// 校验 Tink 目标设备是否存在及权限 (普通 SK 只能推给自己绑定的设备，Admin 可推给任意设备)
	for _, devID := range req.Devices {
		dev, err := h.store.GetDevice(devID)
		if err != nil {
			JSONErrorF(w, r, http.StatusNotFound, i18n.MsgDeviceNotFoundFmt, devID)
			return
		}
		if sk != nil && !sk.IsAdmin() && dev.KeyID != sk.ID {
			JSONErrorF(w, r, http.StatusForbidden, i18n.MsgDeviceNotOwnedFmt, devID)
			return
		}
	}

	var msgID uint64
	var err error
	tinkDispatched := 0
	var msgCreatedAt any

	// 1. 若有 Tink 设备，则执行持久化与本地 SSE 广播
	if len(req.Devices) > 0 {
		msgID, err = h.store.NextMessageID()
		if err != nil {
			JSONError(w, r, http.StatusInternalServerError, i18n.MsgGenerateMessageIDFailed)
			return
		}

		keyID := ""
		if sk != nil {
			keyID = sk.ID
		}

		msg := &store.Message{
			ID:      msgID,
			KeyID:   keyID,
			Devices: req.Devices,
			Group:   req.Group,
			Title:   req.Title,
			Body:    req.Body,
			URL:     req.URL,
			Sound:   req.Sound,
			Payload: req.Payload,
		}

		if err := h.store.SaveMessage(msg); err != nil {
			JSONErrorF(w, r, http.StatusInternalServerError, i18n.MsgPersistErrorFmt, err.Error())
			return
		}

		tinkDispatched = h.hub.DispatchMessage(msg)
		msgCreatedAt = msg.CreatedAt
	}

	// 2. 若有 Bark 设备，则调用上游 Bark 服务并发转发
	barkDispatched := 0
	var barkErrors []string
	if len(req.BarkDevices) > 0 {
		settings, err := h.store.GetSettings()
		if err != nil || !settings.BarkRelayEnabled || settings.BarkServerURL == "" {
			barkErrors = append(barkErrors, "bark relay is disabled or bark_server_url not configured")
		} else {
			sound := req.Sound
			barkDispatched, barkErrors = DispatchBarkPush(
				settings.BarkServerURL,
				req.BarkDevices,
				req.Title,
				req.Body,
				req.Group,
				req.URL,
				sound,
				req.BarkParams,
			)
		}
	}

	resData := map[string]any{
		"dispatched_tink": tinkDispatched,
		"dispatched_bark": barkDispatched,
	}
	if msgID > 0 {
		resData["id"] = msgID
		resData["created_at"] = msgCreatedAt
	}
	if len(barkErrors) > 0 {
		resData["bark_errors"] = barkErrors
	}

	JSONSuccess(w, http.StatusCreated, resData)
}

// EventsSSE GET /api/v1/events
func (h *Handler) EventsSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, i18n.Localize(r, i18n.SSEStreamingUnsupported), http.StatusInternalServerError)
		return
	}

	sk := r.Context().Value(ContextKeySecretKey).(*store.SecretKey)
	deviceID := r.Header.Get("X-Device-ID")
	if deviceID == "" {
		deviceID = r.URL.Query().Get("device_id")
	}
	if deviceID == "" {
		http.Error(w, i18n.Localize(r, i18n.SSEDeviceIDRequired), http.StatusBadRequest)
		return
	}

	// 校验设备是否存在及归属
	dev, err := h.store.GetDevice(deviceID)
	if err != nil {
		http.Error(w, i18n.Localize(r, i18n.SSEDeviceNotRegistered), http.StatusUnauthorized)
		return
	}
	if sk != nil && !sk.IsAdmin() && dev.KeyID != sk.ID {
		http.Error(w, i18n.Localize(r, i18n.SSEUnauthorizedDevice), http.StatusForbidden)
		return
	}

	_ = h.store.UpdateDeviceConnected(deviceID, true)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher.Flush()

	keyID := ""
	if sk != nil {
		keyID = sk.ID
	}
	conn := sse.NewConnection(keyID, deviceID, w, flusher)
	h.hub.Register(conn)
	defer func() {
		h.hub.UnregisterConnection(conn)
	}()

	// 检查 Last-Event-ID 重放离线消息
	lastEventIDStr := r.Header.Get("Last-Event-ID")
	if lastEventIDStr == "" {
		lastEventIDStr = r.URL.Query().Get("last_event_id")
	}

	if lastEventIDStr != "" {
		if lastID, err := strconv.ParseUint(lastEventIDStr, 10, 64); err == nil {
			missed, err := h.store.GetMessagesAfter(deviceID, lastID, 200)
			if err == nil {
				for _, m := range missed {
					if err := conn.SendMessage(m); err != nil {
						return
					}
				}
			}
		}
	}

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case <-conn.CloseChan:
			return
		case msg := <-conn.MsgChan:
			if err := conn.SendMessage(msg); err != nil {
				return
			}
		}
	}
}

// ListKeys GET /api/v1/keys
func (h *Handler) ListKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := h.store.ListKeys()
	if err != nil {
		JSONError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	type keyResp struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Role      string `json:"role"`
		IsMaster  bool   `json:"is_master"` // 兼容老前端字段
		Enabled   uint8  `json:"enabled"`
		CreatedAt int64  `json:"created_at"`
		LastUsed  int64  `json:"last_used,omitempty"`
	}

	res := make([]keyResp, 0, len(keys))
	for _, k := range keys {
		res = append(res, keyResp{
			ID:        k.ID,
			Name:      k.Name,
			Role:      k.Role,
			IsMaster:  k.IsAdmin(),
			Enabled:   k.Enabled,
			CreatedAt: k.CreatedAt,
			LastUsed:  k.LastUsed,
		})
	}

	JSONSuccess(w, http.StatusOK, res)
}

// CreateKey POST /api/v1/keys
func (h *Handler) CreateKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Name = "default"
	}
	if req.Name == "" {
		req.Name = "default"
	}
	if req.Role != "admin" {
		req.Role = "user"
	}

	rawToken, err := auth.GenerateToken()
	if err != nil {
		JSONError(w, r, http.StatusInternalServerError, i18n.MsgGenerateSecretKeyFailed)
		return
	}

	key := &store.SecretKey{
		ID:        "sk_" + rawToken[len(rawToken)-8:],
		Name:      req.Name,
		Role:      req.Role,
		Hash:      store.HashToken(rawToken),
		Enabled:   1,
		CreatedAt: time.Now().UnixMilli(),
	}

	if err := h.store.CreateKey(key); err != nil {
		JSONError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	JSONSuccess(w, http.StatusCreated, map[string]any{
		"token":      rawToken, // 兼容字段
		"secret_key": rawToken, // 仅在创建时返回一次明文 Secret Key
		"id":         key.ID,
		"name":       key.Name,
		"role":       key.Role,
		"is_master":  key.IsAdmin(),
		"enabled":    key.Enabled,
		"created_at": key.CreatedAt,
	})
}

// UpdateKey PUT /api/v1/keys/:id
func (h *Handler) UpdateKey(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		JSONError(w, r, http.StatusBadRequest, i18n.MsgMissingKeyID)
		return
	}
	keyID := parts[3]

	key, err := h.store.GetKeyByID(keyID)
	if err != nil {
		JSONError(w, r, http.StatusNotFound, i18n.MsgSecretKeyNotFound)
		return
	}

	var req struct {
		Name    *string `json:"name"`
		Enabled *uint8  `json:"enabled"`
		Role    *string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, r, http.StatusBadRequest, i18n.MsgInvalidJSON)
		return
	}

	if req.Name != nil && *req.Name != "" {
		key.Name = *req.Name
	}
	if req.Role != nil && (*req.Role == "admin" || *req.Role == "user") {
		key.Role = *req.Role
	}
	if req.Enabled != nil {
		// Admin 密钥禁止禁用以防系统锁死
		if key.IsAdmin() && *req.Enabled == 0 {
			JSONError(w, r, http.StatusBadRequest, i18n.MsgAdminKeyCannotDisable)
			return
		}
		key.Enabled = *req.Enabled
	}

	if err := h.store.UpdateKey(key); err != nil {
		JSONError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	JSONSuccess(w, http.StatusOK, map[string]any{
		"id":        key.ID,
		"name":      key.Name,
		"role":      key.Role,
		"is_master": key.IsAdmin(),
		"enabled":   key.Enabled,
		"last_used": key.LastUsed,
	})
}

// DeleteKey DELETE /api/v1/keys/:id
func (h *Handler) DeleteKey(w http.ResponseWriter, r *http.Request) {
	currentKey := r.Context().Value(ContextKeySecretKey).(*store.SecretKey)

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		JSONError(w, r, http.StatusBadRequest, i18n.MsgMissingKeyID)
		return
	}
	keyID := parts[3]

	key, err := h.store.GetKeyByID(keyID)
	if err != nil {
		JSONError(w, r, http.StatusNotFound, i18n.MsgSecretKeyNotFound)
		return
	}

	// 自删保护：禁止删除当前发起请求的 Key
	if key.ID == currentKey.ID {
		JSONError(w, r, http.StatusBadRequest, i18n.MsgActiveKeyCannotDelete)
		return
	}

	// Admin 保护：禁止删除 Admin 密钥
	if key.IsAdmin() {
		JSONError(w, r, http.StatusBadRequest, i18n.MsgAdminKeyCannotDelete)
		return
	}

	if err := h.store.DeleteKey(keyID); err != nil {
		JSONError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	JSONSuccess(w, http.StatusOK, map[string]any{"id": keyID, "deleted": 1})
}
