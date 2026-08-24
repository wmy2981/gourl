import { afterEach, describe, expect, it, vi } from 'vitest'
import { api, setServerConfig } from '../lib/api'

// Minimal ReadableStream stub over pre-canned chunks; jsdom has
// TextDecoder but the SSE path needs res.body.getReader().
function streamOf(chunks: string[]) {
  let i = 0
  return {
    getReader: () => ({
      read: () =>
        i < chunks.length
          ? Promise.resolve({ done: false, value: new TextEncoder().encode(chunks[i++]) })
          : Promise.resolve({ done: true, value: undefined }),
      cancel: () => {},
    }),
  }
}

const sseFrame = (rec: unknown) => `event: log\ndata: ${JSON.stringify(rec)}\n\n`

afterEach(() => {
  setServerConfig(null)
  vi.unstubAllGlobals()
  vi.useRealTimers()
})

describe('api.logStream (app mode, fetch-parsed SSE)', () => {
  it('delivers frames and fires onOpen once connected', async () => {
    setServerConfig({ url: 'http://server:8080', token: 't' })
    const fetchMock = vi.fn(async () =>
      new Response(JSON.stringify({ configured: true }), { status: 200 }) as never,
    )
    // Response bodies cannot carry a custom stream; stub fetch with a plain
    // object shaped like what the code consumes.
    fetchMock.mockImplementation(async () => ({ ok: true, status: 200, body: streamOf([sseFrame({ message: 'hi', level: 'info', time: 't' })]) }) as never)
    vi.stubGlobal('fetch', fetchMock)

    const onLog = vi.fn()
    const onOpen = vi.fn()
    const es = api.logStream(onLog, undefined, onOpen)
    await vi.waitFor(() => expect(onLog).toHaveBeenCalledWith(expect.objectContaining({ message: 'hi' })))
    expect(onOpen).toHaveBeenCalledOnce()
    es.close()
  })

  it('reconnects automatically after the stream ends and resets via onOpen', async () => {
    vi.useFakeTimers()
    setServerConfig({ url: 'http://server:8080', token: 't' })
    // First connection dies immediately (done right away), second one works.
    let calls = 0
    const fetchMock = vi.fn(async () => {
      calls++
      if (calls === 1) return { ok: true, status: 200, body: streamOf([]) } as never
      return { ok: true, status: 200, body: streamOf([sseFrame({ message: 'back', level: 'info', time: 't' })]) } as never
    })
    vi.stubGlobal('fetch', fetchMock)

    const onLog = vi.fn()
    let errors = 0
    let opens = 0
    const es = api.logStream(
      onLog,
      () => errors++,
      () => opens++,
    )
    // First connection fails → error, then the backoff schedules attempt 2.
    await vi.waitFor(() => expect(errors).toBe(1))
    expect(fetchMock).toHaveBeenCalledTimes(1)
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2100)
    })
    await vi.waitFor(() => expect(onLog).toHaveBeenCalledWith(expect.objectContaining({ message: 'back' })))
    // Only the second connection succeeded; with real timers the first
    // attempt's onopen never fires because its stream ends before the
    // browser would emit one (fetch has no onopen at all).
    expect(opens).toBeGreaterThanOrEqual(1)
    expect(fetchMock).toHaveBeenCalledTimes(2)
    es.close()
    // close() must stop the retry loop for good.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10000)
    })
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })

  async function act(fn: () => Promise<void>) {
    await fn()
  }

  it('close() during backoff cancels the pending retry', async () => {
    vi.useFakeTimers()
    setServerConfig({ url: 'http://server:8080', token: 't' })
    const fetchMock = vi.fn(async () => ({ ok: false, status: 503, body: null }) as never)
    vi.stubGlobal('fetch', fetchMock)

    const es = api.logStream(vi.fn())
    await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1))
    es.close()
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10000)
    })
    expect(fetchMock).toHaveBeenCalledTimes(1) // no second attempt after close
  })
})
