package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mrasong/tink/server/internal/api"
	"github.com/mrasong/tink/server/internal/sse"
	"github.com/mrasong/tink/server/internal/store"
)

func setupTestServer(t *testing.T) (http.Handler, *store.Store, func()) {
	tmpDir, err := os.MkdirTemp("", "tink-api-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	dbPath := filepath.Join(tmpDir, "test.db")
	st, err := store.Open(dbPath)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("failed to open test store: %v", err)
	}

	rawToken := "sk-tink-testtoken123"
	_ = st.CreateKey(&store.SecretKey{
		ID:        "admin_tok1",
		Name:      "Admin Key",
		Role:      "admin",
		Enabled:   1,
		CreatedAt: time.Now().UnixMilli(),
		Hash:      store.HashToken(rawToken),
	})

	hub := sse.NewHub()
	mux := http.NewServeMux()
	handler := api.RegisterRoutes(mux, st, hub)

	cleanup := func() {
		hub.CloseAll()
		_ = st.Close()
		_ = os.RemoveAll(tmpDir)
	}

	return handler, st, cleanup
}

func TestAPIEndToEnd(t *testing.T) {
	handler, _, cleanup := setupTestServer(t)
	defer cleanup()

	// 1. Ping Check
	reqPing := httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil)
	recPing := httptest.NewRecorder()
	handler.ServeHTTP(recPing, reqPing)
	if recPing.Code != http.StatusOK {
		t.Fatalf("/api/v1/ping check expected 200, got %d", recPing.Code)
	}
	var pingResp struct {
		Message    string `json:"message"`
		Version    string `json:"version"`
		ServerTime int64  `json:"st"`
	}
	if err := json.NewDecoder(recPing.Body).Decode(&pingResp); err != nil {
		t.Fatalf("/api/v1/ping decode json error: %v", err)
	}
	if pingResp.Message != "pong" {
		t.Fatalf("/api/v1/ping expected message 'pong', got %+v", pingResp)
	}
	if pingResp.Version != "dev" {
		t.Fatalf("/api/v1/ping expected version 'dev', got '%s'", pingResp.Version)
	}
	if pingResp.ServerTime <= 0 {
		t.Fatalf("/api/v1/ping expected positive st, got %d", pingResp.ServerTime)
	}

	// 2. Register Device
	devBody := []byte(`{"id":"dev-macbook","name":"MacBook Air"}`)
	reqDev := httptest.NewRequest(http.MethodPost, "/api/v1/devices", bytes.NewBuffer(devBody))
	reqDev.Header.Set("Authorization", "Bearer sk-tink-testtoken123")
	reqDev.Header.Set("Content-Type", "application/json")
	recDev := httptest.NewRecorder()
	handler.ServeHTTP(recDev, reqDev)
	if recDev.Code != http.StatusOK {
		t.Fatalf("register device expected 200, got %d: %s", recDev.Code, recDev.Body.String())
	}
	var devEnvelope struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			ID     string `json:"id"`
			Status uint8  `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recDev.Body.Bytes(), &devEnvelope); err != nil || devEnvelope.Code != 0 {
		t.Fatalf("register device invalid envelope: %v, body: %s", err, recDev.Body.String())
	}
	if devEnvelope.Data.ID != "dev-macbook" {
		t.Fatalf("expected device id 'dev-macbook', got '%s'", devEnvelope.Data.ID)
	}
	if devEnvelope.Data.Status != 0 {
		t.Fatalf("expected newly registered device API status 0, got %d", devEnvelope.Data.Status)
	}

	// 3. Send Message (带目标设备)
	msgBody := []byte(`{"title":"Hello Tink","body":"First test message","url":"https://example.com","devices":["dev-macbook"]}`)
	reqMsg := httptest.NewRequest(http.MethodPost, "/api/v1/messages", bytes.NewBuffer(msgBody))
	reqMsg.Header.Set("Authorization", "Bearer sk-tink-testtoken123")
	reqMsg.Header.Set("Content-Type", "application/json")
	recMsg := httptest.NewRecorder()
	handler.ServeHTTP(recMsg, reqMsg)
	if recMsg.Code != http.StatusCreated {
		t.Fatalf("send message expected 201, got %d: %s", recMsg.Code, recMsg.Body.String())
	}

	var msgEnvelope struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			ID         float64 `json:"id"`
			Dispatched int     `json:"dispatched"`
			CreatedAt  int64   `json:"created_at"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recMsg.Body.Bytes(), &msgEnvelope); err != nil || msgEnvelope.Code != 0 {
		t.Fatalf("failed to decode send message response envelope: %v", err)
	}
	if msgEnvelope.Data.ID != 1 {
		t.Fatalf("expected message id 1, got %v", msgEnvelope.Data.ID)
	}

	// 4. Send Message without devices should be rejected (禁止广播)
	badMsgBody := []byte(`{"title":"Broadcast attempt","body":"Should fail"}`)
	reqBadMsg := httptest.NewRequest(http.MethodPost, "/api/v1/messages", bytes.NewBuffer(badMsgBody))
	reqBadMsg.Header.Set("Authorization", "Bearer sk-tink-testtoken123")
	reqBadMsg.Header.Set("Content-Type", "application/json")
	recBadMsg := httptest.NewRecorder()
	handler.ServeHTTP(recBadMsg, reqBadMsg)
	if recBadMsg.Code != http.StatusBadRequest {
		t.Fatalf("send message without devices expected 400, got %d: %s", recBadMsg.Code, recBadMsg.Body.String())
	}
}

func TestMultiSecretKeyManagementAndPermissions(t *testing.T) {
	handler, _, cleanup := setupTestServer(t)
	defer cleanup()

	// 1. 使用 Admin Key 创建一个普通 User 角色 Secret Key (如用于 GitHub CI)
	createReqBody := []byte(`{"name":"GitHub CI","role":"user"}`)
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/keys", bytes.NewBuffer(createReqBody))
	reqCreate.Header.Set("Authorization", "Bearer sk-tink-testtoken123")
	reqCreate.Header.Set("Content-Type", "application/json")
	recCreate := httptest.NewRecorder()
	handler.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("create key expected 201, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var createEnvelope struct {
		Code int            `json:"code"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(recCreate.Body.Bytes(), &createEnvelope); err != nil || createEnvelope.Code != 0 {
		t.Fatalf("decode create key response: %v, body: %s", err, recCreate.Body.String())
	}
	ciKeyRaw := createEnvelope.Data["secret_key"].(string)
	ciKeyID := createEnvelope.Data["id"].(string)
	if ciKeyRaw == "" || ciKeyID == "" {
		t.Fatalf("expected non-empty key and id")
	}

	// 2. 普通 Secret Key 注册设备，测试设备隔离与定向推送
	devCIBody := []byte(`{"id":"dev-ci-runner","name":"CI Runner"}`)
	reqRegCI := httptest.NewRequest(http.MethodPost, "/api/v1/devices", bytes.NewBuffer(devCIBody))
	reqRegCI.Header.Set("Authorization", "Bearer "+ciKeyRaw)
	reqRegCI.Header.Set("Content-Type", "application/json")
	recRegCI := httptest.NewRecorder()
	handler.ServeHTTP(recRegCI, reqRegCI)
	if recRegCI.Code != http.StatusOK {
		t.Fatalf("register device with CI key expected 200, got %d", recRegCI.Code)
	}

	// 2.1 使用新生成的 Secret Key 发送消息 (定向到 dev-ci-runner)
	msgBody := []byte(`{"title":"Deploy Success","body":"CI passed","devices":["dev-ci-runner"]}`)
	reqMsg := httptest.NewRequest(http.MethodPost, "/api/v1/messages", bytes.NewBuffer(msgBody))
	reqMsg.Header.Set("Authorization", "Bearer "+ciKeyRaw)
	reqMsg.Header.Set("Content-Type", "application/json")
	recMsg := httptest.NewRecorder()
	handler.ServeHTTP(recMsg, reqMsg)
	if recMsg.Code != http.StatusCreated {
		t.Fatalf("send message with secret key expected 201, got %d: %s", recMsg.Code, recMsg.Body.String())
	}

	// 2.2 验证 GET /api/v1/me 身份鉴权
	reqMeCI := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	reqMeCI.Header.Set("Authorization", "Bearer "+ciKeyRaw)
	recMeCI := httptest.NewRecorder()
	handler.ServeHTTP(recMeCI, reqMeCI)
	if recMeCI.Code != http.StatusOK {
		t.Fatalf("get me with secret key expected 200, got %d", recMeCI.Code)
	}
	var meEnvelope struct {
		Code int            `json:"code"`
		Data map[string]any `json:"data"`
	}
	_ = json.Unmarshal(recMeCI.Body.Bytes(), &meEnvelope)
	if meEnvelope.Code != 0 || meEnvelope.Data["role"].(string) != "user" || meEnvelope.Data["id"].(string) != ciKeyID {
		t.Fatalf("expected role user and id %s, got %v", ciKeyID, meEnvelope)
	}

	// CI Key 查设备：应只能查到 dev-ci-runner
	reqDevsCI := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	reqDevsCI.Header.Set("Authorization", "Bearer "+ciKeyRaw)
	recDevsCI := httptest.NewRecorder()
	handler.ServeHTTP(recDevsCI, reqDevsCI)
	var devsCIEnvelope struct {
		Code int              `json:"code"`
		Data []map[string]any `json:"data"`
	}
	_ = json.Unmarshal(recDevsCI.Body.Bytes(), &devsCIEnvelope)
	if devsCIEnvelope.Code != 0 || len(devsCIEnvelope.Data) != 1 || devsCIEnvelope.Data[0]["id"].(string) != "dev-ci-runner" {
		t.Fatalf("expected CI key to only see 1 device (dev-ci-runner), got %v", devsCIEnvelope)
	}

	// Admin Key 查设备：可以看到所有设备 (dev-ci-runner)
	reqDevsMaster := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	reqDevsMaster.Header.Set("Authorization", "Bearer sk-tink-testtoken123")
	recDevsMaster := httptest.NewRecorder()
	handler.ServeHTTP(recDevsMaster, reqDevsMaster)
	var devsMasterEnvelope struct {
		Code int              `json:"code"`
		Data []map[string]any `json:"data"`
	}
	_ = json.Unmarshal(recDevsMaster.Body.Bytes(), &devsMasterEnvelope)
	if devsMasterEnvelope.Code != 0 || len(devsMasterEnvelope.Data) < 1 {
		t.Fatalf("expected Admin key to see devices, got %v", devsMasterEnvelope)
	}

	// 3. 普通 Secret Key 尝试访问 Key 管理接口应被拒绝 (403 Forbidden)
	reqForbidden := httptest.NewRequest(http.MethodGet, "/api/v1/keys", nil)
	reqForbidden.Header.Set("Authorization", "Bearer "+ciKeyRaw)
	recForbidden := httptest.NewRecorder()
	handler.ServeHTTP(recForbidden, reqForbidden)
	if recForbidden.Code != http.StatusForbidden {
		t.Fatalf("secret key accessing key list expected 403, got %d", recForbidden.Code)
	}

	// 4. Admin Key 查看 Key 列表
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/keys", nil)
	reqList.Header.Set("Authorization", "Bearer sk-tink-testtoken123")
	recList := httptest.NewRecorder()
	handler.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("list keys expected 200, got %d", recList.Code)
	}
	var listEnvelope struct {
		Code int              `json:"code"`
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listEnvelope); err != nil || listEnvelope.Code != 0 {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listEnvelope.Data) != 2 {
		t.Fatalf("expected 2 keys (admin + ci), got %d", len(listEnvelope.Data))
	}

	// 5. 禁用该 Secret Key (enabled: 0)
	disableBody := []byte(`{"enabled":0}`)
	reqDisable := httptest.NewRequest(http.MethodPut, "/api/v1/keys/"+ciKeyID, bytes.NewBuffer(disableBody))
	reqDisable.Header.Set("Authorization", "Bearer sk-tink-testtoken123")
	reqDisable.Header.Set("Content-Type", "application/json")
	recDisable := httptest.NewRecorder()
	handler.ServeHTTP(recDisable, reqDisable)
	if recDisable.Code != http.StatusOK {
		t.Fatalf("disable key expected 200, got %d: %s", recDisable.Code, recDisable.Body.String())
	}

	// 6. 已禁用的 Secret Key 发送消息应被拦截 (401)
	reqDisabledMsg := httptest.NewRequest(http.MethodPost, "/api/v1/messages", bytes.NewBuffer(msgBody))
	reqDisabledMsg.Header.Set("Authorization", "Bearer "+ciKeyRaw)
	reqDisabledMsg.Header.Set("Content-Type", "application/json")
	recDisabledMsg := httptest.NewRecorder()
	handler.ServeHTTP(recDisabledMsg, reqDisabledMsg)
	if recDisabledMsg.Code != http.StatusUnauthorized {
		t.Fatalf("send message with disabled key expected 401, got %d", recDisabledMsg.Code)
	}

	// 7. 删除/吊销该 Secret Key
	reqDel := httptest.NewRequest(http.MethodDelete, "/api/v1/keys/"+ciKeyID, nil)
	reqDel.Header.Set("Authorization", "Bearer sk-tink-testtoken123")
	recDel := httptest.NewRecorder()
	handler.ServeHTTP(recDel, reqDel)
	if recDel.Code != http.StatusOK {
		t.Fatalf("delete key expected 200, got %d", recDel.Code)
	}

	// 8. 验证防自删保护: Admin Key 不能删除自身
	reqDelSelf := httptest.NewRequest(http.MethodDelete, "/api/v1/keys/admin_tok1", nil)
	reqDelSelf.Header.Set("Authorization", "Bearer sk-tink-testtoken123")
	recDelSelf := httptest.NewRecorder()
	handler.ServeHTTP(recDelSelf, reqDelSelf)
	if recDelSelf.Code != http.StatusBadRequest {
		t.Fatalf("delete active key expected 400, got %d", recDelSelf.Code)
	}
}

func TestDashboardDisabled(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tink-api-disabled-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open test store: %v", err)
	}
	defer st.Close()

	hub := sse.NewHub()
	defer hub.CloseAll()

	mux := http.NewServeMux()
	handler := api.RegisterRoutes(mux, st, hub, api.Config{EnableDashboard: false})

	// 1. / 应该返回 404
	reqRoot := httptest.NewRequest(http.MethodGet, "/", nil)
	recRoot := httptest.NewRecorder()
	handler.ServeHTTP(recRoot, reqRoot)
	if recRoot.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for /, got %d", recRoot.Code)
	}

	// 2. /dashboard/ 应该返回 404
	reqDash := httptest.NewRequest(http.MethodGet, "/dashboard/", nil)
	recDash := httptest.NewRecorder()
	handler.ServeHTTP(recDash, reqDash)
	if recDash.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for /dashboard/, got %d", recDash.Code)
	}

	// 3. /api/v1/keys 应该返回 404
	reqKeys := httptest.NewRequest(http.MethodGet, "/api/v1/keys", nil)
	recKeys := httptest.NewRecorder()
	handler.ServeHTTP(recKeys, reqKeys)
	if recKeys.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for /api/v1/keys, got %d", recKeys.Code)
	}

	// 4. 但 /api/v1/ping 与消息接口依然可用
	reqPing := httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil)
	recPing := httptest.NewRecorder()
	handler.ServeHTTP(recPing, reqPing)
	if recPing.Code != http.StatusOK {
		t.Fatalf("expected 200 for /api/v1/ping, got %d", recPing.Code)
	}
}

func TestCustomDashboardRoute(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tink-api-custom-route-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open test store: %v", err)
	}
	defer st.Close()

	hub := sse.NewHub()
	defer hub.CloseAll()

	mux := http.NewServeMux()
	handler := api.RegisterRoutes(mux, st, hub, api.Config{
		EnableDashboard: true,
		DashboardRoute:  "my-panel",
	})

	// 1. 自定义私密路径下，访问根路径 / 应该直接返回 404 (禁止重定向暴露目标路径)
	reqRoot := httptest.NewRequest(http.MethodGet, "/", nil)
	recRoot := httptest.NewRecorder()
	handler.ServeHTTP(recRoot, reqRoot)
	if recRoot.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for / to prevent route exposure, got %d", recRoot.Code)
	}

	// 2. 访问 /my-panel 应 301 重定向到 /my-panel/
	reqPanelNoSlash := httptest.NewRequest(http.MethodGet, "/my-panel", nil)
	recPanelNoSlash := httptest.NewRecorder()
	handler.ServeHTTP(recPanelNoSlash, reqPanelNoSlash)
	if recPanelNoSlash.Code != http.StatusMovedPermanently {
		t.Fatalf("expected 301 for /my-panel, got %d", recPanelNoSlash.Code)
	}
	if loc := recPanelNoSlash.Header().Get("Location"); loc != "/my-panel/" {
		t.Fatalf("expected redirect to /my-panel/, got %s", loc)
	}

	// 3. 访问 /my-panel/ 应正确返回 200 页面 HTML
	reqPanel := httptest.NewRequest(http.MethodGet, "/my-panel/", nil)
	recPanel := httptest.NewRecorder()
	handler.ServeHTTP(recPanel, reqPanel)
	if recPanel.Code != http.StatusOK {
		t.Fatalf("expected 200 for /my-panel/, got %d", recPanel.Code)
	}
	if !strings.Contains(recPanel.Body.String(), "<!doctype html>") && !strings.Contains(recPanel.Body.String(), "<html") {
		t.Fatalf("expected html response, got %s", recPanel.Body.String())
	}

	// 4. 原 /dashboard 与 /dashboard/ 应返回 404 (隔离原路径)
	reqOldDash := httptest.NewRequest(http.MethodGet, "/dashboard/", nil)
	recOldDash := httptest.NewRecorder()
	handler.ServeHTTP(recOldDash, reqOldDash)
	if recOldDash.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for /dashboard/, got %d", recOldDash.Code)
	}
}

func TestSettingsAPI(t *testing.T) {
	handler, _, cleanup := setupTestServer(t)
	defer cleanup()

	adminToken := "sk-tink-testtoken123"

	// 1. GET /api/v1/settings 默认配置
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	reqGet.Header.Set("Authorization", "Bearer "+adminToken)
	recGet := httptest.NewRecorder()
	handler.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recGet.Code, recGet.Body.String())
	}

	var resp struct {
		Code int                  `json:"code"`
		Data store.SystemSettings `json:"data"`
	}
	_ = json.NewDecoder(recGet.Body).Decode(&resp)
	if resp.Data.BarkRelayEnabled {
		t.Errorf("expected default BarkRelayEnabled to be false")
	}

	// 2. PUT /api/v1/settings 更新配置
	updateBody := `{"bark_relay_enabled":true,"bark_server_url":"https://api.day.app","bark_route_path":"/bark-relay"}`
	reqPut := httptest.NewRequest(http.MethodPut, "/api/v1/settings", strings.NewReader(updateBody))
	reqPut.Header.Set("Authorization", "Bearer "+adminToken)
	reqPut.Header.Set("Content-Type", "application/json")
	recPut := httptest.NewRecorder()
	handler.ServeHTTP(recPut, reqPut)
	if recPut.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recPut.Code, recPut.Body.String())
	}

	var putResp struct {
		Code int                  `json:"code"`
		Data store.SystemSettings `json:"data"`
	}
	_ = json.NewDecoder(recPut.Body).Decode(&putResp)
	if !putResp.Data.BarkRelayEnabled {
		t.Errorf("expected BarkRelayEnabled to be true after update")
	}
	if putResp.Data.BarkRoutePath != "/bark-relay" {
		t.Errorf("expected BarkRoutePath /bark-relay, got %s", putResp.Data.BarkRoutePath)
	}
}

type roundTripFunc func(req *http.Request) *http.Response

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func TestSendMessageBarkAndTink(t *testing.T) {
	var receivedBarkReqs []map[string]any
	var reqMutex sync.Mutex

	mockTransport := roundTripFunc(func(req *http.Request) *http.Response {
		if req.URL.Path == "/push" {
			var body map[string]any
			_ = json.NewDecoder(req.Body).Decode(&body)
			reqMutex.Lock()
			receivedBarkReqs = append(receivedBarkReqs, body)
			reqMutex.Unlock()
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"code":200,"message":"success"}`)),
				Header:     make(http.Header),
			}
		}
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(strings.NewReader(`{"code":404}`)),
			Header:     make(http.Header),
		}
	})

	mockClient := &http.Client{
		Transport: mockTransport,
	}

	handler, st, cleanup := setupTestServer(t)
	defer cleanup()

	// 注入 mock client
	api.SetBarkHTTPClient(mockClient)
	defer api.SetBarkHTTPClient(nil)

	// 开启 Bark Relay 并指向虚拟 upstream
	_ = st.UpdateSettings(&store.SystemSettings{
		BarkRelayEnabled: true,
		BarkServerURL:    "https://api.day.app",
		BarkRoutePath:    "/bark-relay",
	})

	// 注册一个本地 Tink 设备
	_ = st.UpsertDevice(&store.Device{
		ID:    "tink-dev-1",
		KeyID: "admin_tok1",
		Name:  "My Mac",
	})

	adminToken := "sk-tink-testtoken123"

	// 1. 同时向 tink devices 和 bark_devices 发送通知，并附带 bark_params 个性化参数
	sendJSON := `{
		"devices": ["tink-dev-1"],
		"bark_devices": ["bark-key-ios1", "bark-key-ios2"],
		"bark_params": {
			"level": "timeSensitive",
			"icon": "https://example.com/icon.png",
			"badge": 3
		},
		"title": "测试双推",
		"body": "双发正文内容"
	}`
	reqMsg := httptest.NewRequest(http.MethodPost, "/api/v1/messages", strings.NewReader(sendJSON))
	reqMsg.Header.Set("Authorization", "Bearer "+adminToken)
	reqMsg.Header.Set("Content-Type", "application/json")
	recMsg := httptest.NewRecorder()
	handler.ServeHTTP(recMsg, reqMsg)

	if recMsg.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", recMsg.Code, recMsg.Body.String())
	}

	var res struct {
		Code int `json:"code"`
		Data struct {
			ID             uint64   `json:"id"`
			DispatchedTink int      `json:"dispatched_tink"`
			DispatchedBark int      `json:"dispatched_bark"`
			BarkErrors     []string `json:"bark_errors"`
		} `json:"data"`
	}
	_ = json.NewDecoder(recMsg.Body).Decode(&res)

	if res.Data.DispatchedBark != 2 {
		t.Errorf("expected dispatched_bark to be 2, got %d, errors: %v", res.Data.DispatchedBark, res.Data.BarkErrors)
	}
	if len(receivedBarkReqs) != 2 {
		t.Errorf("expected 2 requests received by mock bark server, got %d", len(receivedBarkReqs))
	} else {
		// 校验 bark_params 个性化参数被正确合并
		req0 := receivedBarkReqs[0]
		if req0["level"] != "timeSensitive" {
			t.Errorf("expected merged level timeSensitive, got %v", req0["level"])
		}
		if req0["icon"] != "https://example.com/icon.png" {
			t.Errorf("expected merged icon https://example.com/icon.png, got %v", req0["icon"])
		}
	}

	// 2. 测试透明反向代理 /bark-relay/...
	proxyReq := httptest.NewRequest(http.MethodPost, "/bark-relay/push", strings.NewReader(`{"device_key":"key-proxy","body":"proxy test"}`))
	recProxy := httptest.NewRecorder()
	handler.ServeHTTP(recProxy, proxyReq)

	if recProxy.Code != http.StatusOK {
		t.Fatalf("expected 200 from proxy, got %d: %s", recProxy.Code, recProxy.Body.String())
	}

	// 3. 关闭 Bark Relay 功能后，发往该路由的请求应实时变更为 404
	_ = st.UpdateSettings(&store.SystemSettings{
		BarkRelayEnabled: false,
		BarkServerURL:    "https://api.day.app",
		BarkRoutePath:    "/bark-relay",
	})
	proxyDisabledReq := httptest.NewRequest(http.MethodPost, "/bark-relay/push", strings.NewReader(`{"device_key":"key-proxy","body":"proxy test"}`))
	recProxyDisabled := httptest.NewRecorder()
	handler.ServeHTTP(recProxyDisabled, proxyDisabledReq)

	if recProxyDisabled.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when bark relay is disabled, got %d", recProxyDisabled.Code)
	}
}
