import request from './request'

// ── Types ─────────────────────────────────────────────────────────────────────

export type IngressMatchKind = 'domain' | 'path'

// IngressUpstream 与后端 model.IngressUpstream 一一对应:
//   - service / raw 互斥
//   - network_pref 控制 Resolver 偏好,空 / 'auto' 等价
//   - override_host / override_port 用于在 Resolver 选择基础上做硬覆盖
//     (典型场景:本机短路改走 docker bridge host,或临时把端口指到旧版本)
export type NetworkPref = '' | 'auto' | 'loopback' | 'private' | 'vpn' | 'tunnel' | 'public'

export interface IngressUpstream {
  type: 'service' | 'raw'
  service_id?: number | null
  raw_url?: string
  network_pref?: NetworkPref
  override_host?: string
  override_port?: number
}

export interface IngressRoute {
  id: number
  ingress_id: number
  sort: number
  path: string
  protocol: string
  upstream: IngressUpstream
  websocket: boolean
  extra: string
  listen_port?: number | null
}

export interface Ingress {
  id: number
  edge_server_id: number
  match_kind: IngressMatchKind
  domain: string
  default_path: string
  cert_id?: number | null
  force_https: boolean
  status: string
  last_applied_at: string | null
  created_at: string
  updated_at: string
}

export interface IngressDetail extends Ingress {
  routes: IngressRoute[]
}

export type ChangeKind = 'add' | 'update' | 'delete'

export interface IngressChange {
  kind: ChangeKind
  path: string
  new_hash?: string
  old_hash?: string
  new_content?: string
  old_content?: string
}

export interface ApplyResult {
  audit_id: number
  changes: IngressChange[]
  no_op: boolean
  output: string
  backup_path: string
  rolled_back: boolean
}

// AuditApply 与后端 model.AuditApply + auditDTO 对齐：
//   - changeset_diff: Reconciler 把每条 Change 压成的人类可读 diff
//   - nginx_t_output: nginx -t / reload 输出合并后的全部 stdout/stderr
//   - actor_username: 后端 1+1 join users 后注入的展示名
export interface AuditApply {
  id: number
  edge_server_id: number
  actor_user_id?: number | null
  actor_username: string
  changeset_diff: string
  rolled_back: boolean
  nginx_t_output: string
  backup_path: string
  duration_ms: number
  created_at: string
}

export interface ServiceOpt {
  id: number
  name: string
  exposed_port: number
}

// ── CRUD ──────────────────────────────────────────────────────────────────────

export function listIngresses(edgeServerId?: number) {
  const params = edgeServerId ? { edge_server_id: edgeServerId } : undefined
  return request.get<never, Ingress[]>('/ingresses', { params })
}

export function getIngress(id: number) {
  return request.get<never, IngressDetail>(`/ingresses/${id}`)
}

export interface CreateIngressBody {
  edge_server_id: number
  match_kind: IngressMatchKind
  domain: string
  default_path?: string
  cert_id?: number | null
  force_https?: boolean
  routes?: RouteBody[]
}

export interface UpdateIngressBody {
  match_kind?: IngressMatchKind
  domain?: string
  default_path?: string
  // 三态：未传字段不动；传 null 清空；传具体 id 替换
  cert_id?: number | null
  force_https?: boolean
}

export interface RouteBody {
  sort?: number
  path: string
  protocol?: string
  upstream: IngressUpstream
  websocket?: boolean
  extra?: string
  listen_port?: number | null
}

export function createIngress(body: CreateIngressBody) {
  return request.post<never, Ingress>('/ingresses', body)
}

export function updateIngress(id: number, body: UpdateIngressBody) {
  return request.put<never, Ingress>(`/ingresses/${id}`, body)
}

export function deleteIngress(id: number) {
  return request.delete<never, null>(`/ingresses/${id}`)
}

export function addIngressRoute(id: number, body: RouteBody) {
  return request.post<never, IngressRoute>(`/ingresses/${id}/routes`, body)
}

export function updateIngressRoute(id: number, rid: number, body: RouteBody) {
  return request.put<never, IngressRoute>(`/ingresses/${id}/routes/${rid}`, body)
}

export function deleteIngressRoute(id: number, rid: number) {
  return request.delete<never, null>(`/ingresses/${id}/routes/${rid}`)
}

// ── Apply / DryRun / Audit ────────────────────────────────────────────────────

export function applyEdge(serverId: number) {
  return request.post<never, ApplyResult>(`/ingresses/edges/${serverId}/apply`, {})
}

export function dryRunEdge(serverId: number) {
  return request.post<never, { changes: IngressChange[] }>(`/ingresses/edges/${serverId}/dry-run`, {})
}

export function listAudit(serverId: number, limit = 50) {
  return request.get<never, AuditApply[]>(`/ingresses/edges/${serverId}/audit`, { params: { limit } })
}

export function listEdgeServices(serverId: number) {
  return request.get<never, ServiceOpt[]>(`/ingresses/services/${serverId}`)
}

// ── Phase Nginx-P3B: 反代 vhost 接管候选 ─────────────────────────────────────

export interface IngressProxyRoute {
  path: string
  proxy_pass: string
  websocket: boolean
  extra: string
}

export interface IngressProxyCandidate {
  config_file: string
  server_name: string
  listen: string
  routes: IngressProxyRoute[]
  fingerprint: string
  already_managed: boolean
}

export interface ImportCandidatesResp {
  candidates: IngressProxyCandidate[]
  errors?: string[]
}

export function listImportCandidates(serverId: number) {
  return request.get<never, ImportCandidatesResp>(
    `/ingresses/edges/${serverId}/import-candidates`,
  )
}
