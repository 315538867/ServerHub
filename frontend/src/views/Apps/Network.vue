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
  // Phase Nginx-P3F: "域名与 SSL"子页(Domain.vue)随 legacy site CRUD 一并下架,
  // 反代/SSL 配置统一归 Ingress 模型管;这里只保留反向视图(Ingresses 子页)
  // 让用户从应用视角看到 "谁在路由我"。
  return [{ value: 'ingresses', label: 'Ingress 路由' }]
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
