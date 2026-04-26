<template>
  <div class="network-wrap">
    <div v-if="app" class="network-topo-wrap">
      <NetworkTopology :app-id="appId" />
    </div>

    <div class="sub-tabs">
      <UiTabs :items="subTabs" :model-value="activeSub" @change="onChange" />
    </div>
    <div class="network-body">
      <router-view />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAppStore } from '@/stores/app'
import NetworkTopology from '@/components/apps/NetworkTopology.vue'
import UiTabs from '@/components/ui/UiTabs.vue'

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()

const appId = computed(() => Number(route.params.appId))
const app = computed(() => appStore.getById(appId.value))

const subTabs = computed(() => {
  const a = app.value
  const list: Array<{ value: string; label: string }> = []
  // 旧 "路由配置" 子页(NginxRoutes.vue)在 P3 已下线,
  // 路由维护改到 /servers/:id/ingresses,但本 tab 提供反向视图(Ingresses 子页)
  // 让用户从应用视角看到 "谁在路由我"。域名 SSL 仅在 site_name 存在时展示。
  if (a?.site_name) list.push({ value: 'domain', label: '域名与 SSL' })
  list.push({ value: 'ingresses', label: 'Ingress 路由' })
  return list
})

const activeSub = computed(() => {
  const seg = route.path.split('/').pop() || ''
  return seg === 'network' ? subTabs.value[0]?.value || 'empty' : seg
})

function onChange(v: string | number) {
  router.push(`/apps/${appId.value}/network/${v}`)
}
</script>

<style scoped>
.network-wrap { display: flex; flex-direction: column; height: 100%; }
.network-topo-wrap {
  padding: var(--space-4) var(--space-6) 0;
  flex-shrink: 0;
}
.sub-tabs {
  padding: 0 var(--space-6);
  background: var(--ui-bg);
  flex-shrink: 0;
}
.network-body { flex: 1; min-height: 0; overflow-y: auto; }
</style>
