import {
  ApiError,
  apiClient,
  getAuthToken,
  setAuthToken,
  streamApiEvents,
  unwrapApiResponse,
  type StreamEvent,
} from './apiClient'

const fetchMock = vi.fn<typeof fetch>()

beforeEach(() => {
  fetchMock.mockReset()
  setAuthToken(null)
  vi.stubGlobal('fetch', fetchMock)
})

afterEach(() => {
  setAuthToken(null)
  vi.unstubAllGlobals()
})

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('apiClient', () => {
  it('calls generated OpenAPI endpoints and unwraps typed JSON', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ status: 'ok' }, 200))

    const health = unwrapApiResponse(await apiClient.GET('/healthz'))

    expect(health.status).toBe('ok')
    const [req] = fetchMock.mock.calls[0] ?? []
    expect(req).toBeInstanceOf(Request)
    expect((req as Request).url).toContain('/healthz')
  })

  it('attaches the Bearer token when configured', async () => {
    setAuthToken('secret-jwt')
    expect(getAuthToken()).toBe('secret-jwt')
    fetchMock.mockResolvedValue(jsonResponse({ status: 'ok' }))

    const ready = unwrapApiResponse(await apiClient.GET('/readyz'))
    expect(ready.status).toBe('ok')

    const [req] = fetchMock.mock.calls[0] ?? []
    expect((req as Request).headers.get('Authorization')).toBe('Bearer secret-jwt')
  })

  it('throws an ApiError built from the backend error shape', async () => {
    const details = [{ field: 'pageSize', reason: 'must be at most 100' }]
    fetchMock.mockResolvedValue(
      jsonResponse({ error: { code: 'UNAVAILABLE', message: 'not ready', details } }, 503),
    )

    const result = await apiClient.GET('/readyz')

    expect(() => unwrapApiResponse(result)).toThrow(ApiError)
    try {
      unwrapApiResponse(result)
    } catch (error: unknown) {
      expect(error).toMatchObject({
        status: 503,
        code: 'UNAVAILABLE',
        message: 'not ready',
        details,
      })
    }
  })

  it('falls back when the error body is not JSON', async () => {
    fetchMock.mockResolvedValue(
      new Response('<html>bad gateway</html>', {
        status: 502,
        statusText: 'Bad Gateway',
        headers: { 'Content-Type': 'text/html' },
      }),
    )

    const result = await apiClient.GET('/readyz')
    try {
      unwrapApiResponse(result)
      expect.unreachable('should have thrown')
    } catch (error: unknown) {
      expect(error).toBeInstanceOf(ApiError)
      expect(error).toMatchObject({ status: 502, code: 'UNKNOWN', message: 'Bad Gateway' })
    }
  })
})

describe('streamApiEvents', () => {
  it('parses SSE event frames and attaches bearer auth', async () => {
    setAuthToken('stream-token')
    const encoder = new TextEncoder()
    const stream = new ReadableStream<Uint8Array>({
      start(controller) {
        controller.enqueue(encoder.encode(': keepalive\n\nevent: delta\ndata: {"token":"hi"}\n\n'))
        controller.enqueue(encoder.encode('data: done\n\n'))
        controller.close()
      },
    })
    fetchMock.mockResolvedValue(
      new Response(stream, { status: 200, headers: { 'Content-Type': 'text/event-stream' } }),
    )

    const received: StreamEvent[] = []
    await streamApiEvents('/healthz', {
      onEvent: (ev) => received.push(ev),
    })

    expect(received).toEqual([
      { event: 'delta', data: '{"token":"hi"}' },
      { event: 'message', data: 'done' },
    ])
    const [, init] = fetchMock.mock.calls[0] ?? []
    expect(new Headers(init?.headers).get('Authorization')).toBe('Bearer stream-token')
  })
})
