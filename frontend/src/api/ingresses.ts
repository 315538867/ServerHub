import request from './request'

// ── Types ─────────────────────────────────────────────────────────────────────

export type IngressMatchKind = 'domain' | 'path'

export interface IngressUpstream {
  type: 'service' | 'raw'
  service_id?: number | null
  port?: number | null
  raw_url?: string
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
  legacy_app_route_id?: number | null
}

export interface Ingress {
  id: number
  edge_server_id: number
  match_kind: IngressMatchKind
  domain: string
  default_path: string
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

export interface AuditApply {
  id: number
  edge_server_id: number
  actor_user_id?: number | null
  actor_username: string
  changeset: string
  rolled_back: boolean
  nginx_t_output: string
  reload_output: string
  backup_path: string
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
  routes?: RouteBody[]
}

export interface UpdateIngressBody {
  match_kind?: IngressMatchKind
  domain?: string
  default_path?: string
}

export interface RouteBody {
  sort?: number
  path: string
  protocol?: string
  upstream: IngressUpstream
  websocket?: boolean
  extra?: string
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
