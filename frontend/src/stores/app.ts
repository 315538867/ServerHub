import { ref } from 'vue'
import { defineStore } from 'pinia'
import { listApps } from '@/api/application'
import type { Application } from '@/types/api'

export const useAppStore = defineStore('app', () => {
  const apps = ref<Application[]>([])
  const loading = ref(false)
  let inflight: Promise<void> | null = null

  async function fetch() {
    if (inflight) return inflight
    loading.value = true
    inflight = (async () => {
      try {
        apps.value = await listApps()
      } finally {
        loading.value = false
        inflight = null
      }
    })()
    return inflight
  }

  // ensure 数据存在；若已加载过且非空则跳过，避免页面切换时重复打 API
  async function ensure() {
    if (apps.value.length > 0 || inflight) return inflight ?? Promise.resolve()
    return fetch()
  }

  function getById(id: number) {
    return apps.value.find(a => a.id === id)
  }

  return { apps, loading, fetch, ensure, getById }
})
