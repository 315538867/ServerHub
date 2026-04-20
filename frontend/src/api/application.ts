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

