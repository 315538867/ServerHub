// M3：旧 Deploy 链路的写端点（create/update/delete/run/rollback/env PUT/webhook）已移除，
// 本文件只保留只读入口，供仪表盘与 Service 详情页展示历史数据。
// 新链路写操作请使用 @/api/release.ts。
import request from './request'
import type { Deploy, DeployLog, DeployVersion } from '@/types/api'

export interface EnvVar { key: string; value: string; secret: boolean }

export function getDeploys() {
  return request.get<never, Deploy[]>('/services')
}

export function getDeploy(id: number) {
  return request.get<never, Deploy>(`/services/${id}`)
}

export function getDeployLogs(id: number, limit = 20) {
  return request.get<never, DeployLog[]>(`/services/${id}/logs`, { params: { limit } })
}

export function getDeployEnv(id: number): Promise<EnvVar[]> {
  return request.get(`/services/${id}/env`)
}

export function getDeployVersions(id: number) {
  return request.get<never, DeployVersion[]>(`/services/${id}/versions`)
}

export function getDeployVersion(id: number, vid: number) {
  return request.get<never, DeployVersion>(`/services/${id}/versions/${vid}`)
}
