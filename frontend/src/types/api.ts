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
  type: 'local' | 'ssh' | ''
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

// Network 描述 server 的可达入口，由 Resolver 在跨机选 upstream 时使用。
// loopback 由后端自动注入，前端只读不编辑（编辑提交时会被后端钩子过滤）。
export type NetworkKind = 'loopback' | 'private' | 'vpn' | 'tunnel' | 'public'

export interface Network {
  kind: NetworkKind
  network_id: string
  address: string
  priority: number
  reachable_from?: number[]
  label?: string
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

// ServerService 是 GET /servers/:id/services 返回的精简 Service 视图，
// 供 Server 详情 "服务" Tab 和 Ingress upstream 下拉使用。
export interface ServerService {
  id: number
  name: string
  type: string
  application_id: number | null
  application_name?: string
  exposed_port: number
  image_name?: string
  work_dir?: string
  last_status?: string
}

export interface Deploy {
  id: number
  name: string
  server_id: number
  type: 'docker' | 'docker-compose' | 'native' | 'static'
  work_dir: string
  image_name: string
  // version management
  desired_version: string
  actual_version: string
  // reconcile
  auto_sync: boolean
  sync_interval: number
  sync_status: '' | 'synced' | 'drifted' | 'syncing' | 'error'
  // status
  last_run_at: string | null
  last_status: '' | 'running' | 'success' | 'failed'
  // Release 三维模型（Phase M1）
  current_release_id?: number | null
  auto_rollback_on_fail?: boolean
  created_at: string
  updated_at: string
}

export interface DeployForm {
  name: string
  server_id: number | null
  type: 'docker' | 'docker-compose' | 'native' | 'static'
  work_dir: string
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
  base_dir: string
  deploy_id: number | null
  db_conn_id: number | null
  expose_mode: 'none' | 'path' | 'site'
  status: 'online' | 'offline' | 'unknown' | 'error'
  created_at: string
  updated_at: string
}

export interface AppDirEntry {
  name: string
  path: string
  status: 'ok' | 'missing'
  size: string
  mtime: string
}

export interface AppNginxRoute {
  id: number
  app_id: number
  path: string
  upstream: string
  extra: string
  sort: number
  created_at: string
  updated_at: string
}

export interface AppNginxConfig {
  expose_mode: 'none' | 'path' | 'site'
  routes: AppNginxRoute[]
}

export interface ApplicationForm {
  name: string
  description: string
  server_id: number
  site_name: string
  domain: string
  container_name: string
  base_dir?: string
  expose_mode: 'none' | 'path' | 'site'
  deploy_id: number | null
  db_conn_id: number | null
}
