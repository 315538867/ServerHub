import request from './request'
import type { LoginResp, User } from '@/types/api'

export function login(username: string, password: string) {
  return request.post<never, LoginResp | TotpRequired>('/auth/login', { username, password })
}

export function totpLogin(tmpToken: string, code: string) {
  return request.post<never, LoginResp>('/auth/totp/login', { tmp_token: tmpToken, code })
}

export function totpSetup() {
  return request.post<never, { secret: string; uri: string }>('/auth/totp/setup')
}

export function totpConfirm(secret: string, code: string) {
  return request.post('/auth/totp/confirm', { secret, code })
}

export function totpDisable() {
  return request.post('/auth/totp/disable')
}

export function updateProfile(oldPassword: string, newUsername?: string, newPassword?: string) {
  return request.put<never, User>('/auth/profile', {
    old_password: oldPassword,
    new_username: newUsername || undefined,
    new_password: newPassword || undefined,
  })
}

export function logout() {
  return request.post('/auth/logout')
}

export function getMe() {
  return request.get<never, User>('/auth/me')
}

export interface TotpRequired {
  require_totp: true
  tmp_token: string
}
