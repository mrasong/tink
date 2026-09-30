package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/mrasong/tink/server/internal/i18n"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/mrasong/tink/server/internal/store"
)

// BarkPushRequest Bark 标准 POST /push 请求结构体
type BarkPushRequest struct {
	DeviceKey string `json:"device_key"`
	Title     string `json:"title,omitempty"`
	Body      string `json:"body,omitempty"`
	Category  string `json:"category,omitempty"`
	Group     string `json:"group,omitempty"`
	Sound     string `json:"sound,omitempty"`
	URL       string `json:"url,omitempty"`
	Badge     *int   `json:"badge,omitempty"`
	Icon      string `json:"icon,omitempty"`
	Level     string `json:"level,omitempty"`
	Volume    *int   `json:"volume,omitempty"`
	Copy      string `json:"copy,omitempty"`
	IsArchive *int   `json:"isArchive,omitempty"`
}

// DefaultHTTPClient 默认使用的 HTTP Client (可在测试中通过 SetBarkHTTPClient 替换)
var (
	barkClientMutex   sync.RWMutex
	defaultBarkClient = &http.Client{
		Timeout: 15 * time.Second,
	}
)

// SetBarkHTTPClient 设置全局 Bark HTTP Client (主要用于单元测试注入 mock transport)
func SetBarkHTTPClient(client *http.Client) {
	barkClientMutex.Lock()
	defer barkClientMutex.Unlock()
	defaultBarkClient = client
}

func getBarkHTTPClient() *http.Client {
	barkClientMutex.RLock()
	defer barkClientMutex.RUnlock()
	if defaultBarkClient == nil {
		return &http.Client{Timeout: 15 * time.Second}
	}
	return defaultBarkClient
}

// BarkRelayProxy 创建透明反向代理处理器
// 截获发往 BarkRoutePath 的请求，StripPrefix 后原封不动转发给 upstream bark-server
func BarkRelayProxy(s *store.Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		settings, err := s.GetSettings()
		if err != nil || !settings.BarkRelayEnabled {
			JSONError(w, r, http.StatusNotFound, i18n.MsgEndpointNotFound)
			return
		}

		upstreamURL, err := url.Parse(settings.BarkServerURL)
		if err != nil || upstreamURL.Host == "" {
			JSONError(w, r, http.StatusBadGateway, i18n.MsgInvalidBarkUpstream)
			return
		}

		client := getBarkHTTPClient()
		proxy := &httputil.ReverseProxy{
			Rewrite: func(pr *httputil.ProxyRequest) {
				pr.SetURL(upstreamURL)
				pr.Out.Host = upstreamURL.Host

				// 移除客户端请求中的路由前缀，例如 /bark-relay/register -> /register
				trimmedPath := strings.TrimPrefix(pr.In.URL.Path, settings.BarkRoutePath)
				if !strings.HasPrefix(trimmedPath, "/") {
					trimmedPath = "/" + trimmedPath
				}
				pr.Out.URL.Path = trimmedPath
			},
			ErrorHandler: func(rw http.ResponseWriter, req *http.Request, proxyErr error) {
				JSONErrorF(rw, req, http.StatusBadGateway, i18n.MsgBarkProxyErrorFmt, proxyErr)
			},
		}
		if client != nil && client.Transport != nil {
			proxy.Transport = client.Transport
		}

		proxy.ServeHTTP(w, r)
	})
}

// SendToBarkClient 发送单个 Bark 消息至上游 bark-server (支持合并自定义 barkParams)
func SendToBarkClient(client *http.Client, upstreamURL string, req *BarkPushRequest, barkParams map[string]any) error {
	baseURL := strings.TrimRight(upstreamURL, "/")
	targetURL := baseURL + "/push"

	// 先将标准 BarkPushRequest 转为 map
	reqMap := make(map[string]any)
	reqData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal bark request: %w", err)
	}
	_ = json.Unmarshal(reqData, &reqMap)

	// 将用户传入的 bark_params 合并进去 (例如 icon, level, badge, copy, autoCopy, isArchive 等)
	for k, v := range barkParams {
		// 避免覆盖 device_key
		if k == "device_key" {
			continue
		}
		reqMap[k] = v
	}

	data, err := json.Marshal(reqMap)
	if err != nil {
		return fmt.Errorf("marshal merged bark payload: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, targetURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create bark request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request bark upstream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodySnippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("bark upstream returned status %d: %s", resp.StatusCode, string(bodySnippet))
	}

	return nil
}

// DispatchBarkPush 并发向多个 bark_devices 派发推送，并附加自定义 barkParams
func DispatchBarkPush(upstreamURL string, barkDevices []string, title, body, group, jumpURL, sound string, barkParams map[string]any) (int, []string) {
	if len(barkDevices) == 0 || upstreamURL == "" {
		return 0, nil
	}

	client := getBarkHTTPClient()

	type result struct {
		deviceKey string
		err       error
	}

	results := make(chan result, len(barkDevices))
	var wg sync.WaitGroup

	for _, devKey := range barkDevices {
		wg.Add(1)
		go func(key string) {
			defer wg.Done()
			req := &BarkPushRequest{
				DeviceKey: key,
				Title:     title,
				Body:      body,
				Group:     group,
				Sound:     sound,
				URL:       jumpURL,
			}
			err := SendToBarkClient(client, upstreamURL, req, barkParams)
			results <- result{deviceKey: key, err: err}
		}(devKey)
	}

	wg.Wait()
	close(results)

	successCount := 0
	var errMsgs []string

	for res := range results {
		if res.err == nil {
			successCount++
		} else {
			errMsgs = append(errMsgs, fmt.Sprintf("%s: %v", res.deviceKey, res.err))
		}
	}

	return successCount, errMsgs
}
