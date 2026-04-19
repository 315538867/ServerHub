import request from './request'

export function getSettings() {
  return request.get<Record<string, string>>('/settings')
}

export function putSettings(data: Record<string, string>) {
  return request.put('/settings', data)
}

export function getAuditLogs(params: { page?: number; size?: number; username?: string; path?: string; status?: string }) {
  return request.get<{ total: number; logs: AuditLog[] }>('/audit/logs', { params })
}

export interface AuditLog {
  id: number
  user_id: number
  username: string
  method: string
  path: string
  status: number
  ip: string
  latency_ms: number
  created_at: string
}
