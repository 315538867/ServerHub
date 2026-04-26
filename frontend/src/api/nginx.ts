import request from './request'

export interface SiteItem {
  name: string
  enabled: boolean
  path: string
}

export function getSites(sid: number) {
  return request.get<never, SiteItem[]>(`/servers/${sid}/nginx/sites`)
}

export function createSite(sid: number, data: {
  name: string; type: 'static' | 'proxy' | 'php'
  domain: string; port?: number; root?: string; proxy?: string
}) {
  return request.post(`/servers/${sid}/nginx/sites`, data)
}

export function getSiteConfig(sid: number, name: string) {
  return request.get<never, { name: string; path: string; content: string }>(`/servers/${sid}/nginx/sites/${encodeURIComponent(name)}/config`)
}

export function putSiteConfig(sid: number, name: string, content: string) {
  return request.put(`/servers/${sid}/nginx/sites/${encodeURIComponent(name)}/config`, { content })
}

export function deleteSite(sid: number, name: string) {
  return request.delete(`/servers/${sid}/nginx/sites/${encodeURIComponent(name)}`)
}

export function enableSite(sid: number, name: string) {
  return request.post(`/servers/${sid}/nginx/sites/${encodeURIComponent(name)}/enable`)
}

export function disableSite(sid: number, name: string) {
  return request.post(`/servers/${sid}/nginx/sites/${encodeURIComponent(name)}/disable`)
}

export function nginxReload(sid: number) {
  return request.post(`/servers/${sid}/nginx/reload`)
}

export function nginxRestart(sid: number) {
  return request.post(`/servers/${sid}/nginx/restart`)
}

// ── Phase Nginx-P3: NginxProfile（多实例配置 + nginx -V probe） ──────────────

export interface NginxProfileEffective {
  nginx_conf_dir: string
  sites_available_dir: string
  sites_enabled_dir: string
  app_locations_dir: string
  streams_conf: string
  cert_dir: string
  nginx_conf_path: string
  hub_site_name: string
  test_cmd: string
  reload_cmd: string
}

export interface NginxProfile {
  edge_server_id: number
  // 用户覆盖项（空 = 用 default）
  nginx_conf_dir: string
  sites_available_dir: string
  sites_enabled_dir: string
  app_locations_dir: string
  streams_conf: string
  cert_dir: string
  nginx_conf_path: string
  hub_site_name: string
  test_cmd: string
  reload_cmd: string
  // 合并后的有效值
  effective: NginxProfileEffective
  // probe 缓存
  binary_path?: string
  version?: string
  build_prefix?: string
  build_conf?: string
  modules?: string[]
  last_probe_at?: string
}

export type NginxProfileUpdate = Pick<NginxProfile,
  | 'nginx_conf_dir' | 'sites_available_dir' | 'sites_enabled_dir'
  | 'app_locations_dir' | 'streams_conf' | 'cert_dir'
  | 'nginx_conf_path' | 'hub_site_name' | 'test_cmd' | 'reload_cmd'>

export function getNginxProfile(sid: number) {
  return request.get<never, NginxProfile>(`/servers/${sid}/nginx/profile`)
}

export function putNginxProfile(sid: number, data: NginxProfileUpdate) {
  return request.put<never, NginxProfile>(`/servers/${sid}/nginx/profile`, data)
}

export function probeNginxProfile(sid: number) {
  return request.post<never, NginxProfile>(`/servers/${sid}/nginx/profile/probe`)
}

export function accessLogsWsUrl(sid: number) {
  return `${location.protocol === 'https:' ? 'wss:' : 'ws:'}//${location.host}/panel/api/v1/servers/${sid}/nginx/logs/access`
}

export function errorLogsWsUrl(sid: number) {
  return `${location.protocol === 'https:' ? 'wss:' : 'ws:'}//${location.host}/panel/api/v1/servers/${sid}/nginx/logs/error`
}
