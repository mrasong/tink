package i18n

// zhMessages 中文词典：消息键常量 -> 译文。
// 以常量键索引：新增/重命名消息时编译器保证两端一致，加新语言仅需仿此新增一份。
var zhMessages = map[string]string{
	// 鉴权与会话
	MsgMissingAuthHeader:     "缺少认证信息 (Authorization 头)",
	MsgInvalidSecretKey:      "Secret Key 无效或已过期",
	MsgCSRFCheckFailed:       "CSRF 校验失败，请刷新页面后重试",
	MsgAdminRoleRequired:     "禁止访问：需要管理员权限",
	MsgRateLimitExceeded:     "请求过于频繁，请稍后再试",
	MsgTooManyFailedAttempts: "失败次数过多，请稍后再试",
	MsgCreateSessionFailed:   "创建会话失败",

	// 通用
	MsgMethodNotAllowed:   "请求方法不被允许",
	MsgEndpointNotFound:   "接口不存在",
	MsgInvalidJSON:        "JSON 格式错误",
	MsgInvalidJSONPayload: "JSON 请求体格式错误",

	// 设备
	MsgDeviceFieldsRequired:  "id 和 name 为必填项",
	MsgMissingDeviceID:       "缺少设备 ID",
	MsgDeviceNotFound:        "设备不存在",
	MsgDeviceDeleteForbidden: "禁止访问：该设备注册于其他 Secret Key，无法删除",
	MsgDeviceNotFoundFmt:     "设备 %s 不存在",
	MsgDeviceNotOwnedFmt:     "设备 %s 不属于当前 Secret Key",

	// 消息发送
	MsgTitleOrBodyRequired:     "title 与 body 至少填写一项",
	MsgPushTargetRequired:      "请至少指定一个目标：Tink 设备填入 'devices'，Bark 设备填入 'bark_devices'",
	MsgGenerateMessageIDFailed: "生成消息 ID 失败",
	MsgPersistErrorFmt:         "写入存储失败：%s",

	// Secret Key 管理
	MsgGenerateSecretKeyFailed: "生成 Secret Key 失败",
	MsgMissingKeyID:            "缺少密钥 ID",
	MsgSecretKeyNotFound:       "Secret Key 不存在",
	MsgAdminKeyCannotDisable:   "不能禁用管理员密钥",
	MsgActiveKeyCannotDelete:   "不能删除当前正在使用的密钥",
	MsgAdminKeyCannotDelete:    "不能删除管理员密钥",

	// 系统设置与 Bark 转发
	MsgGetSettingsErrorFmt:    "获取系统设置失败：%s",
	MsgUpdateSettingsErrorFmt: "更新系统设置失败：%s",
	MsgInvalidBarkUpstream:    "Bark 上游服务地址无效",
	MsgBarkProxyErrorFmt:      "Bark 转发失败：%v",

	// SSE 原始错误负载
	SSEStreamingUnsupported: `{"error":"当前服务不支持流式响应"}`,
	SSEDeviceIDRequired:     `{"error":"需要 X-Device-ID 头或 device_id 查询参数"}`,
	SSEDeviceNotRegistered:  `{"error":"设备尚未注册"}`,
	SSEUnauthorizedDevice:   `{"error":"设备未授权"}`,
}
