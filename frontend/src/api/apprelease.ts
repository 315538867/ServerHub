// Phase M3: AppReleaseSet API 客户端
//
// CRUD 走 axios（统一拦截）；Apply/Rollback 走原生 fetch + ReadableStream，
// 因为后端接口是 POST + SSE（EventSource 不支持 POST），且需要 Bearer token。
import request from './request'
import type {
  AppReleaseSet,
  ApplySseEvent,
  ApplySseEventName,
} from '@/types/apprelease'

const BASE = '/panel/api/v1'

// ── CRUD ──────────────────────────────────────────────────────────

export function listAppReleaseSets(appId: number) {
  return request.get<never, AppReleaseSet[]>(`/apps/${appId}/release-sets`)
}

export function getAppReleaseSet(appId: number, rsId: number) {
  return request.get<never, AppReleaseSet>(
    `/apps/${appId}/release-sets/${rsId}`,
  )
}

export function createAppReleaseSet(
  appId: number,
  payload: { label?: string; note?: string },
) {
  return request.post<never, AppReleaseSet>(
    `/apps/${appId}/release-sets`,
    payload,
  )
}

// ── SSE 流式 Apply / Rollback ─────────────────────────────────────

export interface SseRunOptions {
  onEvent: (e: ApplySseEvent) => void
  signal?: AbortSignal
}

async function runSseStream(url: string, opts: SseRunOptions) {
  const token = localStorage.getItem('token')
  const resp = await fetch(url, {
    method: 'POST',
    headers: {
      Authorization: token ? `Bearer ${token}` : '',
      'Content-Type': 'application/json',
      Accept: 'text/event-stream',
    },
    signal: opts.signal,
  })
  if (!resp.ok || !resp.body) {
    throw new Error(`SSE 启动失败 HTTP ${resp.status}`)
  }
  const reader = resp.body.getReader()
  const decoder = new TextDecoder()
  let buf = ''
  // SSE 帧以 "\n\n" 分隔，每帧形如：
  //   event: <name>\ndata: <json>\n\n
  while (true) {
    const { value, done } = await reader.read()
    if (done) break
    buf += decoder.decode(value, { stream: true })
    let idx: number
    while ((idx = buf.indexOf('\n\n')) >= 0) {
      const frame = buf.slice(0, idx)
      buf = buf.slice(idx + 2)
      const ev = parseFrame(frame)
      if (ev) opts.onEvent(ev)
    }
  }
  // flush 残尾（极少见，但兼容服务端不补 \n\n 的情况）
  if (buf.trim()) {
    const ev = parseFrame(buf)
    if (ev) opts.onEvent(ev)
  }
}

function parseFrame(raw: string): ApplySseEvent | null {
  let name: string | null = null
  let data: string | null = null
  for (const line of raw.split('\n')) {
    if (line.startsWith('event:')) name = line.slice(6).trim()
    else if (line.startsWith('data:')) data = line.slice(5).trim()
  }
  if (!name) return null
  let payload: unknown = {}
  if (data) {
    try {
      payload = JSON.parse(data)
    } catch {
      payload = { raw: data }
    }
  }
  return { name: name as ApplySseEventName, data: payload as never } as ApplySseEvent
}

export function applyAppReleaseSetSSE(
  appId: number,
  rsId: number,
  opts: SseRunOptions,
) {
  return runSseStream(
    `${BASE}/apps/${appId}/release-sets/${rsId}/apply`,
    opts,
  )
}

export function rollbackAppReleaseSetSSE(
  appId: number,
  rsId: number,
  opts: SseRunOptions,
) {
  return runSseStream(
    `${BASE}/apps/${appId}/release-sets/${rsId}/rollback`,
    opts,
  )
}
