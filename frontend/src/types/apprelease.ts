// Phase M3: AppReleaseSet（App 级 Release 组合）前端类型
// 与后端 model/app_release_set.go 保持一致

export type AppReleaseSetStatus =
  | 'draft'
  | 'applying'
  | 'success'
  | 'partial'
  | 'failed'
  | 'rolled_back'

export type AppReleaseSummaryStatus = 'success' | 'failed' | 'skipped'

export interface AppReleaseSet {
  id: number
  application_id: number
  label: string
  items: string // JSON [{service_id,release_id}]
  note: string
  status: AppReleaseSetStatus
  created_by: string
  applied_at: string | null
  last_summary: string // JSON [{service_id,run_id?,status,error?}]
  created_at: string
  updated_at: string
}

export interface AppReleaseSetItem {
  service_id: number
  release_id: number
}

export interface AppReleaseSummaryItem {
  service_id: number
  run_id?: number
  status: AppReleaseSummaryStatus
  error?: string
}

// ── SSE 事件 payload 类型 ───────────────────────────────────────────

export interface SetStartedEvent {
  set_id: number
  total: number
  items?: AppReleaseSetItem[]
}

export interface ServiceStartedEvent {
  service_id: number
  release_id: number
  idx: number
  total: number
}

export interface ServiceLineEvent {
  service_id: number
  line: string
}

export interface ServiceDoneEvent {
  service_id: number
  run_id?: number
  status: AppReleaseSummaryStatus
  duration_sec?: number
  error?: string
}

export interface SetDoneEvent {
  status: AppReleaseSetStatus
  summary: AppReleaseSummaryItem[]
  success?: number
  failed?: number
}

export interface SseErrorEvent {
  error: string
  code?: string
}

export type ApplySseEventName =
  | 'set_started'
  | 'service_started'
  | 'service_line'
  | 'service_done'
  | 'set_done'
  | 'error'
  | 'done'

// 联合事件：用 name 区分
export type ApplySseEvent =
  | { name: 'set_started'; data: SetStartedEvent }
  | { name: 'service_started'; data: ServiceStartedEvent }
  | { name: 'service_line'; data: ServiceLineEvent }
  | { name: 'service_done'; data: ServiceDoneEvent }
  | { name: 'set_done'; data: SetDoneEvent }
  | { name: 'error'; data: SseErrorEvent }
  | { name: 'done'; data: Record<string, unknown> }
