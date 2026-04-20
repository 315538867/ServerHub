<template>
  <div class="ops-wrap" :class="{ 'ops-wrap--terminal': activeSub === 'terminal' }">
    <!-- 始终可见的容器状态条（终端全屏除外） -->
    <div v-if="app?.container_name && activeSub !== 'terminal'" class="ops-statusbar-wrap">
      <ops-status-bar :app-id="appId" />
    </div>

    <div class="sub-tabs">
      <t-tabs :value="activeSub" @change="onChange">
        <t-tab-panel
          v-for="t in subTabs"
          :key="t.value"
          :value="t.value"
          :label="t.label"
        />
      </t-tabs>
    </div>
    <div class="ops-body" :class="{ 'ops-body--terminal': activeSub === 'terminal' }">
      <router-view />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAppStore } from '@/stores/app'
import OpsStatusBar from '@/components/apps/OpsStatusBar.vue'

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()

const appId = computed(() => Number(route.params.appId))
const app = computed(() => appStore.getById(appId.value))

const subTabs = computed(() => {
  const a = app.value
  const list: Array<{ value: string; label: string }> = []
  if (a?.container_name) list.push({ value: 'service', label: '容器控制' })
  list.push({ value: 'logs', label: '日志' })
  list.push({ value: 'terminal', label: '终端' })
  return list
})

const activeSub = computed(() => {
  const seg = route.path.split('/').pop() || ''
  return seg === 'ops' ? subTabs.value[0]?.value || 'logs' : seg
})

function onChange(v: string | number) {
  router.push(`/apps/${appId.value}/ops/${v}`)
}
</script>

<style scoped>
.ops-wrap {
  display: flex;
  flex-direction: column;
  height: 100%;
}
.ops-wrap--terminal { overflow: hidden; }

.ops-statusbar-wrap {
  padding: var(--sh-space-md) var(--sh-space-lg) 0;
  flex-shrink: 0;
}

.sub-tabs {
  padding: 0 var(--sh-space-lg);
  background: var(--sh-bg);
  border-bottom: 1px solid var(--sh-border);
  flex-shrink: 0;
}
.sub-tabs :deep(.t-tabs__nav) { border-bottom: none; }
.sub-tabs :deep(.t-tabs__nav-container) { padding: 0; }
.ops-body { flex: 1; overflow-y: auto; min-height: 0; }
.ops-body--terminal { overflow: hidden; padding: 0; }
</style>
