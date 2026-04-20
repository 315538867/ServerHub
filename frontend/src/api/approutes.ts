import request from './request'
import type { AppNginxConfig, AppNginxRoute } from '@/types/api'

export function getAppNginx(appId: number) {
  return request.get<never, AppNginxConfig>(`/apps/${appId}/nginx`)
}

export function setExposeMode(appId: number, mode: 'none' | 'path' | 'site') {
  return request.put<never, null>(`/apps/${appId}/nginx/mode`, { mode })
}

export function addRoute(appId: number, data: { path: string; upstream: string; extra?: string; sort?: number }) {
  return request.post<never, AppNginxRoute>(`/apps/${appId}/nginx/routes`, data)
}

export function updateRoute(appId: number, rid: number, data: { path: string; upstream: string; extra?: string; sort?: number }) {
  return request.put<never, AppNginxRoute>(`/apps/${appId}/nginx/routes/${rid}`, data)
}

export function deleteRoute(appId: number, rid: number) {
  return request.delete<never, null>(`/apps/${appId}/nginx/routes/${rid}`)
}

export function applyNginx(appId: number) {
  return request.post<never, { output: string }>(`/apps/${appId}/nginx/apply`, {})
}
