import request from './request'
import type { Deploy } from '@/types/api'
import type {
  Artifact,
  CreateArtifactPayload,
  CreateConfigSetPayload,
  CreateEnvSetPayload,
  CreateReleasePayload,
  ConfigFileSet,
  DeployRun,
  EnvVarSet,
  Release,
  TriggerSource,
} from '@/types/release'

// Service 基础信息(只读)。M2 之前住在 @/api/deploy,deploy 包退役后归这里——
// /services/:id 子树本就是 Release 链路的入口,基础查询和 Release 操作放在
// 同一个 API 模块里语义最自洽。Deploy 类型是 Service 的历史别名,字段未变。
export function getService(sid: number) {
  return request.get<never, Deploy>(`/services/${sid}`)
}

// Release
export function listReleases(sid: number) {
  return request.get<never, Release[]>(`/services/${sid}/releases`)
}

export function createRelease(sid: number, payload: CreateReleasePayload) {
  return request.post<never, Release>(`/services/${sid}/releases`, payload)
}

export function applyRelease(sid: number, rid: number, trigger: TriggerSource = 'manual') {
  return request.post<never, DeployRun>(`/services/${sid}/releases/${rid}/apply`, {
    trigger_source: trigger,
  })
}

// Artifact
export function listArtifacts(sid: number) {
  return request.get<never, Artifact[]>(`/services/${sid}/artifacts`)
}

export function createArtifact(sid: number, payload: CreateArtifactPayload) {
  return request.post<never, Artifact>(`/services/${sid}/artifacts`, payload)
}

export function uploadArtifact(sid: number, file: File) {
  const fd = new FormData()
  fd.append('file', file)
  return request.post<never, Artifact>(`/services/${sid}/artifacts`, fd, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
}

export function probeArtifact(sid: number, aid: number) {
  return request.post<never, { probed: boolean; msg?: string }>(
    `/services/${sid}/artifacts/${aid}/probe`,
  )
}

// EnvVarSet
export function listEnvSets(sid: number) {
  return request.get<never, EnvVarSet[]>(`/services/${sid}/env-sets`)
}

export function createEnvSet(sid: number, payload: CreateEnvSetPayload) {
  return request.post<never, EnvVarSet>(`/services/${sid}/env-sets`, payload)
}

// ConfigFileSet
export function listConfigSets(sid: number) {
  return request.get<never, ConfigFileSet[]>(`/services/${sid}/config-sets`)
}

export function createConfigSet(sid: number, payload: CreateConfigSetPayload) {
  return request.post<never, ConfigFileSet>(`/services/${sid}/config-sets`, payload)
}

// DeployRun
export function listDeployRuns(sid: number) {
  return request.get<never, DeployRun[]>(`/services/${sid}/deploy-runs`)
}

export function getDeployRun(sid: number, runid: number) {
  return request.get<never, DeployRun>(`/services/${sid}/deploy-runs/${runid}`)
}

// Settings
export function setAutoRollback(sid: number, enabled: boolean) {
  return request.patch<never, void>(`/services/${sid}/settings/auto-rollback`, { enabled })
}
