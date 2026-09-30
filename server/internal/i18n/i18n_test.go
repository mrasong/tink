package i18n

import (
	"net/http/httptest"
	"testing"
)

// allKeys 汇总全部消息键常量，用于词典覆盖率守卫。
var allKeys = []string{
	MsgMissingAuthHeader, MsgInvalidSecretKey, MsgCSRFCheckFailed, MsgAdminRoleRequired,
	MsgRateLimitExceeded, MsgTooManyFailedAttempts, MsgCreateSessionFailed,
	MsgMethodNotAllowed, MsgEndpointNotFound, MsgInvalidJSON, MsgInvalidJSONPayload,
	MsgDeviceFieldsRequired, MsgMissingDeviceID, MsgDeviceNotFound, MsgDeviceDeleteForbidden,
	MsgDeviceNotFoundFmt, MsgDeviceNotOwnedFmt,
	MsgTitleOrBodyRequired, MsgPushTargetRequired, MsgGenerateMessageIDFailed, MsgPersistErrorFmt,
	MsgGenerateSecretKeyFailed, MsgMissingKeyID, MsgSecretKeyNotFound,
	MsgAdminKeyCannotDisable, MsgActiveKeyCannotDelete, MsgAdminKeyCannotDelete,
	MsgGetSettingsErrorFmt, MsgUpdateSettingsErrorFmt, MsgInvalidBarkUpstream, MsgBarkProxyErrorFmt,
	SSEStreamingUnsupported, SSEDeviceIDRequired, SSEDeviceNotRegistered, SSEUnauthorizedDevice,
}

func TestZhDictionaryCoversAllKeys(t *testing.T) {
	for _, key := range allKeys {
		if _, ok := zhMessages[key]; !ok {
			t.Errorf("zh dictionary missing translation for key %q", key)
		}
	}
	for key := range zhMessages {
		found := false
		for _, k := range allKeys {
			if k == key {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("zh dictionary has orphan key %q with no constant", key)
		}
	}
}

func TestRequestLang(t *testing.T) {
	cases := map[string]string{
		"":                        "en",
		"zh-CN,zh;q=0.9,en;q=0.8": "zh",
		"zh-TW":                   "zh",
		"en-US,en;q=0.9":          "en",
		"de,fr":                   "en",
	}
	for header, want := range cases {
		r := httptest.NewRequest("GET", "/", nil)
		if header != "" {
			r.Header.Set("Accept-Language", header)
		}
		if got := RequestLang(r); got != want {
			t.Errorf("Accept-Language %q: got %s want %s", header, got, want)
		}
	}
}

func TestLocalize(t *testing.T) {
	zh := httptest.NewRequest("GET", "/", nil)
	zh.Header.Set("Accept-Language", "zh-CN")
	en := httptest.NewRequest("GET", "/", nil)
	en.Header.Set("Accept-Language", "en")

	if got := Localize(zh, MsgInvalidSecretKey); got != "Secret Key 无效或已过期" {
		t.Errorf("zh localize got %q", got)
	}
	if got := Localize(en, MsgInvalidSecretKey); got != MsgInvalidSecretKey {
		t.Errorf("en localize should keep contract string, got %q", got)
	}
	// 词典未收录的透传消息保持原样
	if got := Localize(zh, "some low-level store error"); got != "some low-level store error" {
		t.Errorf("unknown key got %q", got)
	}
	if got := Localize(nil, MsgInvalidSecretKey); got != MsgInvalidSecretKey {
		t.Errorf("nil request got %q", got)
	}
}
