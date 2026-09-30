// Package i18n 集中管理 API 错误消息键与各语言词典。
// 代码中一律引用本包常量 (如 i18n.MsgDeviceNotFound)，禁止手写英文消息字面量：
// 新增语言只需在 translations 中补一份以常量键的词典，拼错常量名会直接编译报错。
package i18n

import (
	"net/http"
	"strings"
)

// 消息键常量。值为英文原文，即对外 API 契约 (macOS 客户端/CLI 依赖)，禁止改动。
const (
	// 鉴权与会话
	MsgMissingAuthHeader     = "missing authorization header"
	MsgInvalidSecretKey      = "invalid or expired secret key"
	MsgCSRFCheckFailed       = "csrf check failed"
	MsgAdminRoleRequired     = "forbidden: admin role required"
	MsgRateLimitExceeded     = "rate limit exceeded"
	MsgTooManyFailedAttempts = "too many failed attempts, retry later"
	MsgCreateSessionFailed   = "failed to create session"

	// 通用
	MsgMethodNotAllowed   = "method not allowed"
	MsgEndpointNotFound   = "endpoint not found"
	MsgInvalidJSON        = "invalid json"
	MsgInvalidJSONPayload = "invalid json payload"

	// 设备
	MsgDeviceFieldsRequired  = "id and name are required"
	MsgMissingDeviceID       = "missing device id"
	MsgDeviceNotFound        = "device not found"
	MsgDeviceDeleteForbidden = "forbidden: cannot delete device registered with another secret key"
	MsgDeviceNotFoundFmt     = "device %s not found"
	MsgDeviceNotOwnedFmt     = "device %s does not belong to your secret key"

	// 消息发送
	MsgTitleOrBodyRequired     = "title or body is required"
	MsgPushTargetRequired      = "at least one target is required: specify 'devices' for Tink clients or 'bark_devices' for Bark clients"
	MsgGenerateMessageIDFailed = "failed to generate message id"
	MsgPersistErrorFmt         = "persist error: %s"

	// Secret Key 管理
	MsgGenerateSecretKeyFailed = "generate secret key failed"
	MsgMissingKeyID            = "missing key id"
	MsgSecretKeyNotFound       = "secret key not found"
	MsgAdminKeyCannotDisable   = "admin key cannot be disabled"
	MsgActiveKeyCannotDelete   = "cannot delete currently active secret key"
	MsgAdminKeyCannotDelete    = "admin key cannot be deleted"

	// 系统设置与 Bark 转发
	MsgGetSettingsErrorFmt    = "failed to get settings: %s"
	MsgUpdateSettingsErrorFmt = "failed to update settings: %s"
	MsgInvalidBarkUpstream    = "invalid upstream bark server url"
	MsgBarkProxyErrorFmt      = "bark relay proxy error: %v"

	// SSE 原始错误负载 (非 Envelope，保持 JSON 字符串)
	SSEStreamingUnsupported = `{"error":"streaming unsupported"}`
	SSEDeviceIDRequired     = `{"error":"X-Device-ID header or device_id query required"}`
	SSEDeviceNotRegistered  = `{"error":"device not registered"}`
	SSEUnauthorizedDevice   = `{"error":"unauthorized device"}`
)

// translations 语言 -> (消息键 -> 译文)。未收录的键 (如底层 err.Error() 透传) 保持英文原样。
var translations = map[string]map[string]string{
	"zh": zhMessages,
}

// RequestLang 解析 Accept-Language：首个匹配 zh* 的语言返回 zh，否则 en (默认)。
// zh-TW/zh-HK 等暂统一按简体处理 (当前仅承诺 zh-cn)。
func RequestLang(r *http.Request) string {
	for _, part := range strings.Split(r.Header.Get("Accept-Language"), ",") {
		tag := strings.ToLower(strings.TrimSpace(strings.Split(strings.TrimSpace(part), ";")[0]))
		if tag == "" {
			continue
		}
		if strings.HasPrefix(tag, "zh") {
			return "zh"
		}
		if strings.HasPrefix(tag, "en") {
			return "en"
		}
	}
	return "en"
}

// Localize 按请求语言翻译消息键。
func Localize(r *http.Request, key string) string {
	if r == nil {
		return key
	}
	lang := RequestLang(r)
	if dict, ok := translations[lang]; ok {
		if msg, ok := dict[key]; ok {
			return msg
		}
	}
	return key
}
