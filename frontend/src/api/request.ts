import axios from 'axios'
import { $message } from '@/utils/discrete'
import { useAuthStore } from '@/stores/auth'
import router from '@/router'

const instance = axios.create({
  baseURL: '/panel/api/v1',
  timeout: 30000,
})

instance.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

instance.interceptors.response.use(
  (res) => {
    const data = res.data as { code: number; msg: string; data: unknown }
    if (data.code !== 0) {
      $message().error(data.msg || 'request failed')
      return Promise.reject(data)
    }
    return data.data as never
  },
  (err) => {
    if (err.response?.status === 401) {
      const url = err.config?.url ?? ''
      const isAuthEndpoint = url.includes('/auth/')
      if (isAuthEndpoint) {
        $message().error(err.response?.data?.msg || '用户名或密码错误')
      } else {
        const authStore = useAuthStore()
        authStore.logout()
        router.push('/login')
      }
    } else {
      $message().error(err.response?.data?.msg || '网络错误')
    }
    return Promise.reject(err)
  },
)

export default instance
