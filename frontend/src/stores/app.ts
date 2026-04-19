import { ref } from 'vue'
import { defineStore } from 'pinia'
import { listApps } from '@/api/application'
import type { Application } from '@/types/api'

export const useAppStore = defineStore('app', () => {
  const apps = ref<Application[]>([])
  const loading = ref(false)

  async function fetch() {
    loading.value = true
    try {
      apps.value = await listApps()
    } finally {
      loading.value = false
    }
  }

  function getById(id: number) {
    return apps.value.find(a => a.id === id)
  }

  return { apps, loading, fetch, getById }
})
