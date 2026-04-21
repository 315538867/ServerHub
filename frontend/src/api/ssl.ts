import request from './request'

export interface SSLCert {
  id: number
  domain: string
  cert_path: string
  key_path: string
  issuer: string
  expires_at: string
  days_left: number
  auto_renew: boolean
}

export function listCerts(serverId: number) {
  return request.get<SSLCert[]>(`/servers/${serverId}/ssl/certs`)
}

export function deleteCert(serverId: number, certId: number) {
  return request.delete(`/servers/${serverId}/ssl/certs/${certId}`)
}

export function uploadCert(serverId: number, body: {
  domain: string; cert: string; key: string; cert_path?: string; key_path?: string
}) {
  return request.post(`/servers/${serverId}/ssl/certs/upload`, body)
}

export function scanCerts(serverId: number) {
  return request.post<{ imported: number }>(`/servers/${serverId}/ssl/certs/scan`)
}

export function requestCertWsUrl(serverId: number, params: { domain: string; webroot?: string; email?: string }, token: string) {
  const q = new URLSearchParams({ token, domain: params.domain })
  if (params.webroot) q.set('webroot', params.webroot)
  if (params.email) q.set('email', params.email)
  return `${location.protocol === 'https:' ? 'wss:' : 'ws:'}//${location.host}/panel/api/v1/servers/${serverId}/ssl/certs/request?${q}`
}

export function renewCertWsUrl(serverId: number, certId: number, token: string) {
  return `${location.protocol === 'https:' ? 'wss:' : 'ws:'}//${location.host}/panel/api/v1/servers/${serverId}/ssl/certs/${certId}/renew?token=${token}`
}
