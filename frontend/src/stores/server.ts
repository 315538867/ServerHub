import { ref } from 'vue'
import { defineStore } from 'pinia'
import { getServers } from '@/api/servers'
import type { Server } from '@/types/api'

export const useServerStore = defineStore('server', () => {
  const servers = ref<Server[]>([])
  const loading = ref(false)

  async function fetch() {
    loading.value = true
    try {
      servers.value = await getServers()
    } finally {
      loading.value = false
    }
  }

  function getById(id: number) {
    return servers.value.find(s => s.id === id)
  }

  return { servers, loading, fetch, getById }
})
