import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { User } from '@/types/api'
import { login as apiLogin, logout as apiLogout } from '@/api/auth'

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string>(localStorage.getItem('token') ?? '')
  const user = ref<User | null>(null)

  function setToken(t: string) {
    token.value = t
    localStorage.setItem('token', t)
  }

  function setTokenAndUser(t: string, u: User) {
    setToken(t)
    user.value = u
  }

  async function login(username: string, password: string) {
    const data = await apiLogin(username, password)
    if (data && 'require_totp' in data && data.require_totp) {
      return data
    }
    const resp = data as { token: string; user: User }
    setToken(resp.token)
    user.value = resp.user
    return data
  }

  function logout() {
    token.value = ''
    user.value = null
    localStorage.removeItem('token')
  }

  return { token, user, login, logout, setToken, setTokenAndUser }
})
