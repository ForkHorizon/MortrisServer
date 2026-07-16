import type { ApiErrorBody } from './types'

export class ApiError extends Error {
  code: string
  status: number
  requestId: string

  constructor(body: ApiErrorBody, status: number) {
    super(body.message)
    this.code = body.code
    this.status = status
    this.requestId = body.request_id
  }
}

function readCookie(name: string): string | null {
  const match = document.cookie.match(new RegExp(`(?:^|; )${name}=([^;]*)`))
  return match ? decodeURIComponent(match[1]) : null
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = {}
  if (body !== undefined) headers['Content-Type'] = 'application/json'
  // Double-submit CSRF (section 10.3): echo the cookie back as a header
  // on every state-changing request. GET/HEAD never need it.
  if (method !== 'GET' && method !== 'HEAD') {
    const csrf = readCookie('csrf_token')
    if (csrf) headers['X-CSRF-Token'] = csrf
  }

  const res = await fetch(path, {
    method,
    headers,
    credentials: 'include',
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })

  if (res.status === 204) return undefined as T

  const text = await res.text()
  const data = text ? JSON.parse(text) : undefined

  if (!res.ok) {
    throw new ApiError(data as ApiErrorBody, res.status)
  }
  return data as T
}

export function apiGet<T>(path: string, params?: Record<string, string | number | undefined>): Promise<T> {
  const query = params
    ? '?' +
      Object.entries(params)
        .filter(([, v]) => v !== undefined && v !== '')
        .map(([k, v]) => `${encodeURIComponent(k)}=${encodeURIComponent(String(v))}`)
        .join('&')
    : ''
  return request<T>('GET', `${path}${query}`)
}

export function apiPost<T>(path: string, body?: unknown): Promise<T> {
  return request<T>('POST', path, body)
}

export function apiDelete<T>(path: string): Promise<T> {
  return request<T>('DELETE', path)
}
