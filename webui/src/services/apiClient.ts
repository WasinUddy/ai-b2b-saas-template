import createClient, { type Middleware } from 'openapi-fetch'
import type { components, paths } from '@/types/api.gen'

export type ApiPaths = paths
export type ApiSchemas = components['schemas']
export type ApiErrorDetail = ApiSchemas['handler.ErrorDetail']

interface ErrorBody {
  error?: { code?: string; message?: string; details?: ApiErrorDetail[] }
}

export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly details: ApiErrorDetail[]

  constructor(status: number, code: string, message: string, details: ApiErrorDetail[] = []) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.details = details
  }
}

let bearerToken: string | null = null

/** Sets or clears the optional Bearer token sent with local API requests. */
export function setAuthToken(token: string | null): void {
  bearerToken = token
}

/** Returns the currently configured Bearer token, if any. */
export function getAuthToken(): string | null {
  return bearerToken
}

const authMiddleware: Middleware = {
  onRequest({ request }) {
    if (bearerToken && !request.headers.has('Authorization')) {
      request.headers.set('Authorization', `Bearer ${bearerToken}`)
    }
    return request
  },
}

const defaultBaseUrl =
  typeof window !== 'undefined' &&
  typeof window.location?.origin === 'string' &&
  window.location.origin.startsWith('http')
    ? window.location.origin
    : 'http://localhost'

/**
 * OpenAPI-typed client for every local backend endpoint declared in api/openapi.yaml.
 * Run `make generate` after changing Go handlers or DTOs to refresh `@/types/api.gen`.
 */
export const apiClient = createClient<paths>({
  baseUrl: defaultBaseUrl,
  headers: { Accept: 'application/json' },
  fetch: (input) => globalThis.fetch(input),
})
apiClient.use(authMiddleware)

function toApiError(response: Response, errorBody: unknown): ApiError {
  const body =
    typeof errorBody === 'object' && errorBody !== null ? (errorBody as ErrorBody) : undefined
  const err = body?.error
  return new ApiError(
    response.status,
    err?.code ?? 'UNKNOWN',
    err?.message ?? (response.statusText || 'Request failed'),
    err?.details,
  )
}

/**
 * Unwraps an `openapi-fetch` result, throwing `ApiError` when the HTTP status is not OK.
 *
 * Example:
 * ```ts
 * const health = unwrapApiResponse(await apiClient.GET('/healthz'))
 * ```
 */
export function unwrapApiResponse<T>(result: { data?: T; error?: unknown; response: Response }): T {
  if (!result.response.ok) {
    throw toApiError(result.response, result.error)
  }
  return result.data as T
}

export interface StreamEvent {
  event: string
  data: string
}

export interface StreamApiOptions {
  method?: 'GET' | 'POST'
  body?: unknown
  headers?: HeadersInit
  signal?: AbortSignal
  onEvent: (event: StreamEvent) => void
}

/**
 * Connects to a local Server-Sent Events endpoint declared in the generated OpenAPI paths.
 * Supports both GET and POST streams (such as LLM token streaming) and sends the Bearer token if set.
 */
export async function streamApiEvents(path: keyof paths, options: StreamApiOptions): Promise<void> {
  const headers = new Headers(options.headers)
  headers.set('Accept', 'text/event-stream')
  if (bearerToken && !headers.has('Authorization')) {
    headers.set('Authorization', `Bearer ${bearerToken}`)
  }
  const hasBody = options.body !== undefined && options.body !== null
  if (hasBody && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }

  const response = await fetch(String(path), {
    method: options.method ?? 'GET',
    headers,
    body: hasBody ? JSON.stringify(options.body) : undefined,
    signal: options.signal,
  })
  if (!response.ok) {
    let parsed: unknown
    try {
      parsed = await response.json()
    } catch {
      // Non-JSON error response; fall back to status text.
    }
    throw toApiError(response, parsed)
  }
  if (!response.body) {
    return
  }

  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

  for (;;) {
    const { done, value } = await reader.read()
    if (done) {
      break
    }
    buffer += decoder.decode(value, { stream: true })
    buffer = flushSSEFrames(buffer, options.onEvent)
  }
  buffer += decoder.decode()
  flushSSEFrames(buffer, options.onEvent)
}

function flushSSEFrames(buffer: string, onEvent: (event: StreamEvent) => void): string {
  const normalized = buffer.replace(/\r\n/g, '\n')
  const frames = normalized.split('\n\n')
  const remainder = frames.pop() ?? ''

  for (const frame of frames) {
    let eventName = 'message'
    const dataLines: string[] = []
    for (const line of frame.split('\n')) {
      if (line === '' || line.startsWith(':')) {
        continue
      }
      if (line.startsWith('event:')) {
        eventName = line.slice('event:'.length).trim()
      } else if (line.startsWith('data:')) {
        dataLines.push(line.slice('data:'.length).trimStart())
      }
    }
    if (dataLines.length > 0) {
      onEvent({ event: eventName, data: dataLines.join('\n') })
    }
  }
  return remainder
}
