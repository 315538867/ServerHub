import request from './request'
import type { Deploy, DeployForm, DeployLog } from '@/types/api'

export function getDeploys() {
  return request.get<never, Deploy[]>('/deploys')
}

export function getDeploy(id: number) {
  return request.get<never, Deploy>(`/deploys/${id}`)
}

export function createDeploy(data: DeployForm) {
  return request.post<never, Deploy>('/deploys', data)
}

export function updateDeploy(id: number, data: Partial<DeployForm>) {
  return request.put<never, Deploy>(`/deploys/${id}`, data)
}

export function deleteDeploy(id: number) {
  return request.delete(`/deploys/${id}`)
}

export function getDeployLogs(id: number, limit = 20) {
  return request.get<never, DeployLog[]>(`/deploys/${id}/logs`, { params: { limit } })
}

export interface EnvVar { key: string; value: string; secret: boolean }

export function getDeployEnv(id: number): Promise<EnvVar[]> {
  return request.get(`/deploys/${id}/env`)
}

export function putDeployEnv(id: number, vars: EnvVar[]) {
  return request.put(`/deploys/${id}/env`, vars)
}

export function getWebhookInfo(id: number): Promise<{ url: string; secret: string }> {
  return request.get(`/deploys/${id}/webhook`)
}
