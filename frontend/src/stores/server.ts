import { ref } from 'vue'
import { defineStore } from 'pinia'
import { getServers } from '@/api/servers'
import type { Server } from '@/types/api'

export const useServerStore = defineStore('server', () => {
  const servers = ref<Server[]>([])
  const loading = ref(false)
  let inflight: Promise<void> | null = null

  async function fetch() {
    if (inflight) return inflight
    loading.value = true
    inflight = (async () => {
      try {
        servers.value = await getServers()
      } finally {
        loading.value = false
        inflight = null
      }
    })()
    return inflight
  }

  async function ensure() {
    if (servers.value.length > 0 || inflight) return inflight ?? Promise.resolve()
    return fetch()
  }

  function getById(id: number) {
    return servers.value.find(s => s.id === id)
  }

  return { servers, loading, fetch, ensure, getById }
})
