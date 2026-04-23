import request from './request'

export interface SetupStatus {
  containerized: boolean
  needs_admin: boolean
}

export function getSetupStatus() {
  return request.get<never, SetupStatus>('/setup/status')
}

export function createAdmin(username: string, password: string) {
  return request.post<never, { username: string }>('/setup/admin', { username, password })
}
