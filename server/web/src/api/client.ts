import { t } from '@/i18n'

export async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers || {})
  if (options.body && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json')
  // 会话 cookie 鉴权时，服务端要求非 GET 请求携带此自定义头 (CSRF 防线)
  const method = (options.method || 'GET').toUpperCase()
  if (method !== 'GET' && method !== 'HEAD' && !headers.has('X-Tink-CSRF'))
    headers.set('X-Tink-CSRF', '1')

  const res = await fetch(path, { ...options, headers, credentials: 'same-origin' })
  const body = await res.json().catch(() => ({}))
  if (body && typeof body === 'object' && 'code' in body) {
    if (body.code !== 0) throw new Error(body.message || t('api.request_failed_code', { code: body.code }))
    return body.data as T
  }
  if ((res.status === 401 || res.status === 403) && path.startsWith('/api/v1/')) {
    throw new Error(body.error || body.message || t('api.auth_failed', { status: res.status }))
  }
  if (!res.ok) throw new Error(body.error || body.message || t('api.request_failed_http', { status: res.status }))
  return body as T
}
