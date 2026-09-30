package api

import (
	"encoding/json"
	"github.com/mrasong/tink/server/internal/i18n"
	"net/http"

	"github.com/mrasong/tink/server/internal/store"
)

// GetSettings GET /api/v1/settings
// 获取当前系统的设置信息 (仅 Admin 可访问)
func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.store.GetSettings()
	if err != nil {
		JSONErrorF(w, r, http.StatusInternalServerError, i18n.MsgGetSettingsErrorFmt, err.Error())
		return
	}

	JSONSuccess(w, http.StatusOK, settings)
}

// UpdateSettings PUT /api/v1/settings
// 更新系统设置 (仅 Admin 可访问)
func (h *Handler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BarkRelayEnabled *bool   `json:"bark_relay_enabled"`
		BarkServerURL    *string `json:"bark_server_url"`
		BarkRoutePath    *string `json:"bark_route_path"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, r, http.StatusBadRequest, i18n.MsgInvalidJSONPayload)
		return
	}

	current, err := h.store.GetSettings()
	if err != nil {
		current = store.DefaultSettings()
	}

	if req.BarkRelayEnabled != nil {
		current.BarkRelayEnabled = *req.BarkRelayEnabled
	}
	if req.BarkServerURL != nil {
		current.BarkServerURL = *req.BarkServerURL
	}
	if req.BarkRoutePath != nil {
		current.BarkRoutePath = *req.BarkRoutePath
	}

	if err := h.store.UpdateSettings(current); err != nil {
		JSONErrorF(w, r, http.StatusInternalServerError, i18n.MsgUpdateSettingsErrorFmt, err.Error())
		return
	}

	// 重新获取规范化后的最新设置
	updated, _ := h.store.GetSettings()
	JSONSuccess(w, http.StatusOK, updated)
}
