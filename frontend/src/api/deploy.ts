import request from './request'
import type { Deploy, DeployForm, DeployLog, DeployVersion } from '@/types/api'

export function getDeploys() {
  return request.get<never, Deploy[]>('/services')
}

export function getDeploy(id: number) {
  return request.get<never, Deploy>(`/services/${id}`)
}

export function createDeploy(data: DeployForm) {
  return request.post<never, Deploy>('/services', data)
}

export function updateDeploy(id: number, data: Partial<DeployForm>) {
  return request.put<never, Deploy>(`/services/${id}`, data)
}

export function deleteDeploy(id: number) {
  return request.delete(`/services/${id}`)
}

export function getDeployLogs(id: number, limit = 20) {
  return request.get<never, DeployLog[]>(`/services/${id}/logs`, { params: { limit } })
}

export interface EnvVar { key: string; value: string; secret: boolean }

export function getDeployEnv(id: number): Promise<EnvVar[]> {
  return request.get(`/services/${id}/env`)
}

export function putDeployEnv(id: number, vars: EnvVar[]) {
  return request.put(`/services/${id}/env`, vars)
}

export function getWebhookInfo(id: number): Promise<{ url: string; secret: string }> {
  return request.get(`/services/${id}/webhook`)
}

export function getDeployVersions(id: number) {
  return request.get<never, DeployVersion[]>(`/services/${id}/versions`)
}

export function getDeployVersion(id: number, vid: number) {
  return request.get<never, DeployVersion>(`/services/${id}/versions/${vid}`)
}
