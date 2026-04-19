export interface ApiResponse<T = unknown> {
  code: number
  msg: string
  data: T
}

export interface PageResult<T> {
  list: T[]
  total: number
  page: number
  size: number
}

export interface User {
  id: number
  username: string
  role: string
  mfa_enabled: boolean
  last_login: string | null
  last_ip: string
  created_at: string
  updated_at: string
}

export interface LoginResp {
  token: string
  user: User
}

export interface HealthData {
  version: string
  uptime: number
  db_status: string
  os: string
  arch: string
}

export interface Server {
  id: number
  name: string
  host: string
  port: number
  username: string
  auth_type: 'password' | 'key'
  remark: string
  status: 'online' | 'offline' | 'unknown'
  last_check_at: string | null
  created_at: string
  updated_at: string
}

export interface ServerForm {
  name: string
  host: string
  port: number
  username: string
  auth_type: 'password' | 'key'
  password?: string
  private_key?: string
  remark?: string
}

export interface Metric {
  id: number
  server_id: number
  cpu: number
  mem: number
  disk: number
  load1: number
  uptime: number
  created_at: string
}

export interface Deploy {
  id: number
  name: string
  server_id: number
  type: 'docker' | 'docker-compose' | 'native'
  work_dir: string
  compose_file: string
  start_cmd: string
  image_name: string
  // version management
  desired_version: string
  actual_version: string
  previous_version: string
  // reconcile
  auto_sync: boolean
  sync_interval: number
  sync_status: '' | 'synced' | 'drifted' | 'syncing' | 'error'
  // status
  last_run_at: string | null
  last_status: '' | 'running' | 'success' | 'failed'
  created_at: string
  updated_at: string
}

export interface DeployForm {
  name: string
  server_id: number | null
  type: 'docker' | 'docker-compose' | 'native'
  work_dir: string
  compose_file?: string
  start_cmd?: string
  image_name?: string
  desired_version?: string
  auto_sync?: boolean
  sync_interval?: number
}

export interface DeployLog {
  id: number
  deploy_id: number
  output: string
  status: 'success' | 'failed'
  duration: number
  created_at: string
}

export interface Application {
  id: number
  name: string
  description: string
  server_id: number
  site_name: string
  domain: string
  container_name: string
  deploy_id: number | null
  db_conn_id: number | null
  status: 'online' | 'offline' | 'unknown' | 'error'
  created_at: string
  updated_at: string
}

export interface ApplicationForm {
  name: string
  description: string
  server_id: number
  site_name: string
  domain: string
  container_name: string
  deploy_id: number | null
  db_conn_id: number | null
}
