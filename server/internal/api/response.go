package api

import (
	"encoding/json"
	"fmt"
	"github.com/mrasong/tink/server/internal/i18n"
	"net/http"
)

// Response 全局统一 API 响应结构 (Envelope)
type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

// JSONSuccess 写入成功的统一结构响应
func JSONSuccess(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(Response{
		Code:    0,
		Message: "ok",
		Data:    data,
	})
}

// JSONError 写入失败/错误的统一结构响应，按请求 Accept-Language 本地化 message
func JSONError(w http.ResponseWriter, r *http.Request, statusCode int, message string) {
	writeError(w, statusCode, i18n.Localize(r, message))
}

// JSONErrorF 面向含动态参数的消息：以英文 format 为词典键翻译模板后再格式化
func JSONErrorF(w http.ResponseWriter, r *http.Request, statusCode int, format string, args ...any) {
	msg := i18n.Localize(r, format)
	if len(args) > 0 {
		msg = fmt.Sprintf(msg, args...)
	}
	writeError(w, statusCode, msg)
}

func writeError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(Response{
		Code:    statusCode,
		Message: message,
		Data:    nil,
	})
}
