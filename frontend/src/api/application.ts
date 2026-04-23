import request from './request'
import type { Application, ApplicationForm, AppDirEntry } from '@/types/api'

export function listApps(serverId?: number) {
  const params = serverId ? { server_id: serverId } : {}
  return request.get<never, Application[]>('/apps', { params })
}

export function getApp(id: number) {
  return request.get<never, Application>(`/apps/${id}`)
}

export function createApp(data: ApplicationForm) {
  return request.post<never, Application>('/apps', data)
}

export function updateApp(id: number, data: ApplicationForm) {
  return request.put<never, Application>(`/apps/${id}`, data)
}

export function deleteApp(id: number) {
  return request.delete(`/apps/${id}`)
}

export function getAppDirs(id: number) {
  return request.get<never, AppDirEntry[]>(`/apps/${id}/dirs`)
}

export function initAppDirs(id: number) {
  return request.post<never, { message: string }>(`/apps/${id}/init-dirs`, {})
}

// 应用实时指标（通过 SSH 调用远端 docker stats）
export interface AppMetrics {
  available: boolean
  reason?: string
  cpu_percent: number
  mem_usage: string
  mem_percent: number
  net_io: string
  block_io: string
  pids: number
  container_id: string
  ts: number
}

export function getAppMetrics(id: number) {
  return request.get<never, AppMetrics>(`/apps/${id}/metrics`)
}

// 应用下挂的 Service 列表（Phase C：1:N 关系）
export interface AppService {
  id: number
  name: string
  type: string
  work_dir: string
  last_status: string
  actual_version: string
  source_kind: string
}

export function listAppServices(id: number) {
  return request.get<never, AppService[]>(`/apps/${id}/services`)
}

export function attachServiceToApp(appId: number, serviceId: number) {
  return request.post<never, { message: string }>(`/apps/${appId}/services/${serviceId}/attach`, {})
}

export function detachServiceFromApp(appId: number, serviceId: number) {
  return request.delete(`/apps/${appId}/services/${serviceId}/attach`)
}

