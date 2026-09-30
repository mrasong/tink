// 会话凭证现由服务端 httpOnly cookie 管理，JS 无法读取。
// 这里仅负责清除历史版本遗留在 localStorage 的明文 Secret Key。
const LEGACY_TOKEN_KEY = 'tink_admin_token'

export const purgeLegacyToken = () => localStorage.removeItem(LEGACY_TOKEN_KEY)
