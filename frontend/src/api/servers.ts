import request from './request'
import type { Server, ServerForm, Metric } from '@/types/api'

export function getServers() {
  return request.get<never, Server[]>('/servers')
}

export function createServer(data: ServerForm) {
  return request.post<never, Server>('/servers', data)
}

export function getServer(id: number) {
  return request.get<never, Server>(`/servers/${id}`)
}

export function updateServer(id: number, data: Partial<ServerForm>) {
  return request.put<never, Server>(`/servers/${id}`, data)
}

export function deleteServer(id: number) {
  return request.delete(`/servers/${id}`)
}

export function testServer(id: number) {
  return request.post<never, { status: string; error?: string }>(`/servers/${id}/test`)
}

export function collectMetrics(id: number) {
  return request.post<never, Metric>(`/servers/${id}/metrics/collect`)
}

export function getMetrics(id: number, limit = 60) {
  return request.get<never, Metric[]>(`/servers/${id}/metrics`, { params: { limit } })
}
