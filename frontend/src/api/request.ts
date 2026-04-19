import axios from 'axios'
import { ElMessage } from 'element-plus'
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
      ElMessage.error(data.msg || 'request failed')
      return Promise.reject(data)
    }
    return data.data as never
  },
  (err) => {
    if (err.response?.status === 401) {
      const url = err.config?.url ?? ''
      const isAuthEndpoint = url.includes('/auth/')
      if (isAuthEndpoint) {
        // 登录/验证接口：显示具体错误
        ElMessage.error(err.response?.data?.msg || '用户名或密码错误')
      } else {
        // 其他接口 401：Token 过期，登出并跳转
        const authStore = useAuthStore()
        authStore.logout()
        router.push('/login')
      }
    } else {
      ElMessage.error(err.response?.data?.msg || '网络错误')
    }
    return Promise.reject(err)
  },
)

export default instance
