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

export function accessLogsWsUrl(sid: number, token: string) {
  return `${location.protocol === 'https:' ? 'wss:' : 'ws:'}//${location.host}/panel/api/v1/servers/${sid}/nginx/logs/access?token=${token}`
}

export function errorLogsWsUrl(sid: number, token: string) {
  return `${location.protocol === 'https:' ? 'wss:' : 'ws:'}//${location.host}/panel/api/v1/servers/${sid}/nginx/logs/error?token=${token}`
}
