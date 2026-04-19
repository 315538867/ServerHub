import request from './request'
import type { Metric } from '@/types/api'

export interface ServerOverview {
  id: number
  name: string
  host: string
  port: number
  status: 'online' | 'offline' | 'unknown'
  last_check_at: string | null
  metric: Metric | null
}

export function getOverview() {
  return request.get<never, ServerOverview[]>('/metrics/overview')
}

export function getServerMetrics(id: number, limit = 60) {
  return request.get<never, Metric[]>(`/servers/${id}/metrics`, { params: { limit } })
}
