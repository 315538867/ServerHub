import request from './request'
import type { Metric } from '@/types/api'

export interface ServerOverview {
  id: number
  name: string
  host: string
  port: number
  // R3 起与 Server 一致,新增 lagging 枚举,源自 derive.ServerStatus
  status: 'online' | 'lagging' | 'offline' | 'unknown'
  last_check_at: string | null
  metric: Metric | null
}

export function getOverview() {
  return request.get<never, ServerOverview[]>('/metrics/overview')
}

export function getServerMetrics(id: number, limit = 60) {
  return request.get<never, Metric[]>(`/servers/${id}/metrics`, { params: { limit } })
}
