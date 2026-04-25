// Release 三维正交模型前端类型定义。
// 对应后端 backend/model/{release,artifact,env_var_set,config_file_set,deploy_run}.go。

export type ArtifactProvider =
  | 'upload'
  | 'script'
  | 'git'
  | 'http'
  | 'docker'
  | 'imported'

export type ReleaseStatus = 'draft' | 'active' | 'rolled_back' | 'archived'

export type DeployRunStatus = 'running' | 'success' | 'failed' | 'rolled_back'

export type TriggerSource =
  | 'manual'
  | 'webhook'
  | 'schedule'
  | 'api'
  | 'auto_rollback'

export interface Artifact {
  id: number
  service_id: number
  provider: ArtifactProvider
  ref: string
  pull_script: string
  checksum: string
  size_bytes: number
  created_at: string
}

export interface EnvVarItem {
  key: string
  value: string
  secret: boolean
}

export interface EnvVarSet {
  id: number
  service_id: number
  label: string
  created_at: string
}

export interface ConfigFileItem {
  name: string
  content_b64: string
  mode: number
}

export interface ConfigFileSet {
  id: number
  service_id: number
  label: string
  files: string
  created_at: string
}

// StartSpec 是 JSON string（数据库 text 列）。
// 前端构造时按 Service.type 决定字段，UI 层可用 StartSpecByType 联合：
export type StartSpec =
  | { type: 'docker'; image?: string; cmd?: string }
  | { type: 'docker-compose'; file_name?: string }
  | { type: 'native'; cmd: string }
  | { type: 'static' }

export interface Release {
  id: number
  service_id: number
  label: string
  artifact_id: number
  env_set_id: number | null
  config_set_id: number | null
  start_spec: string
  note: string
  created_by: string
  status: ReleaseStatus
  created_at: string
}

export interface DeployRun {
  id: number
  service_id: number
  release_id: number
  status: DeployRunStatus
  trigger_source: TriggerSource
  started_at: string
  finished_at: string | null
  duration_sec: number
  output: string
  rollback_from_run_id: number | null
}

export interface CreateReleasePayload {
  label?: string
  artifact_id: number
  env_set_id?: number | null
  config_set_id?: number | null
  start_spec?: Record<string, unknown>
  note?: string
}

export interface CreateArtifactPayload {
  provider: Exclude<ArtifactProvider, 'upload' | 'imported'>
  ref?: string
  pull_script?: string
}

export interface CreateEnvSetPayload {
  label?: string
  vars: EnvVarItem[]
}

export interface CreateConfigSetPayload {
  label?: string
  files: ConfigFileItem[]
}
