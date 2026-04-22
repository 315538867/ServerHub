import request from './request'

export interface SetupStatus {
  containerized: boolean
  needs_admin: boolean
  needs_local_server: boolean
  host_gateway?: string
}

export interface InitLocalResp {
  public_key: string
  host_gateway: string
  target_user: string
  command: string
  expires_at: string
}

export function getSetupStatus() {
  return request.get<never, SetupStatus>('/setup/status')
}

export function createAdmin(username: string, password: string) {
  return request.post<never, { username: string }>('/setup/admin', { username, password })
}

export function initLocal(targetUser: string) {
  return request.post<never, InitLocalResp>('/setup/local/init', { target_user: targetUser })
}

export function activateLocal() {
  return request.post<never, { server_id: number; name: string }>('/setup/local/activate', {})
}
