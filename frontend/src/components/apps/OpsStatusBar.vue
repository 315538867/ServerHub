<template>
  <div v-if="app?.container_name" class="ops-bar" :class="barStateClass">
    <div class="bar-section bar-section--status">
      <span class="bar-dot" />
      <span class="bar-name" :title="app.container_name">{{ app.container_name }}</span>
      <span class="bar-state">{{ stateText }}</span>
    </div>

    <div class="bar-section bar-section--meta">
      <template v-if="container">
        <div class="bar-kv">
          <span class="bar-k">镜像</span>
          <code class="bar-v" :title="container.image">{{ container.image }}</code>
        </div>
        <div class="bar-kv">
          <span class="bar-k">状态</span>
          <span class="bar-v">{{ container.status }}</span>
        </div>
        <div v-if="container.ports" class="bar-kv bar-kv--ports">
          <span class="bar-k">端口</span>
          <span class="bar-v" :title="container.ports">{{ container.ports }}</span>
        </div>
      </template>
      <div v-else-if="!loading" class="bar-kv bar-kv--missing">
        <span>服务器上未找到该容器</span>
      </div>
    </div>

    <div class="bar-section bar-section--actions">
      <UiButton
        v-if="container && container.state !== 'running'"
        variant="success" size="sm"
        :loading="acting === 'start'"
        @click="doAction('start')"
      >
        <template #icon><Play :size="13" /></template>
        启动
      </UiButton>
      <UiButton
        v-if="container?.state === 'running'"
        variant="warning" size="sm"
        :loading="acting === 'stop'"
        @click="doAction('stop')"
      >
        <template #icon><Square :size="13" /></template>
        停止
      </UiButton>
      <UiButton
        v-if="container"
        variant="secondary" size="sm"
        :loading="acting === 'restart'"
        @click="doAction('restart')"
      >
        <template #icon><RotateCw :size="13" /></template>
        重启
      </UiButton>
      <UiButton variant="ghost" size="sm" :loading="loading" @click="refresh()" title="刷新">
        <template #icon><RefreshCw :size="13" /></template>
      </UiButton>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted, onBeforeUnmount, watch } from 'vue'
import { useMessage } from 'naive-ui'
import { Play, Square, RotateCw, RefreshCw } from 'lucide-vue-next'
import { useAppStore } from '@/stores/app'
import { getContainers, containerAction } from '@/api/docker'
import type { ContainerItem } from '@/api/docker'
import UiButton from '@/components/ui/UiButton.vue'

const props = defineProps<{ appId: number }>()
const emit = defineEmits<{ (e: 'changed'): void }>()

const message = useMessage()
const appStore = useAppStore()
const app = computed(() => appStore.getById(props.appId))
const serverId = computed(() => app.value?.server_id ?? 0)

const container = ref<ContainerItem | null>(null)
const loading = ref(false)
const acting = ref<'start' | 'stop' | 'restart' | ''>('')

let timer: ReturnType<typeof setInterval> | null = null

const stateText = computed(() => {
  if (!container.value) return loading.value ? '加载中…' : '未找到'
  const s = container.value.state
  return ({ running: '运行中', paused: '已暂停', exited: '已停止', created: '已创建', restarting: '重启中', dead: '异常' } as Record<string, string>)[s] || s
})

const barStateClass = computed(() => {
  const s = container.value?.state
  if (s === 'running') return 'ops-bar--ok'
  if (s === 'paused' || s === 'restarting') return 'ops-bar--warn'
  if (!container.value) return 'ops-bar--unknown'
  return 'ops-bar--down'
})

async function refresh(silent = false) {
  if (!serverId.value || !app.value?.container_name) return
  if (!silent) loading.value = true
  try {
    const list = await getContainers(serverId.value)
    container.value = list.find(c => c.names === app.value!.container_name || c.names.split(',').includes(app.value!.container_name)) || null
  } catch { /* silent */ }
  finally { if (!silent) loading.value = false }
}

async function doAction(action: 'start' | 'stop' | 'restart') {
  if (!container.value) return
  acting.value = action
  try {
    await containerAction(serverId.value, container.value.id, action)
    message.success(({ start: '已启动', stop: '已停止', restart: '已重启' } as Record<string, string>)[action])
    await refresh()
    emit('changed')
  } catch { message.error('操作失败') }
  finally { acting.value = '' }
}

function startPoll() { stopPoll(); timer = setInterval(() => refresh(true), 10000) }
function stopPoll() { if (timer) { clearInterval(timer); timer = null } }

watch(() => props.appId, () => { refresh() })
onMounted(() => { refresh(); startPoll() })
onBeforeUnmount(stopPoll)
</script>

<style scoped>
.ops-bar {
  display: flex;
  align-items: center;
  gap: var(--space-5);
  padding: var(--space-2) var(--space-4);
  background: var(--ui-bg-2);
  border: 1px solid var(--ui-border);
  border-radius: var(--radius-md);
  margin: 0 0 var(--space-3);
  border-left: 3px solid var(--ui-border);
  flex-wrap: wrap;
}
.ops-bar--ok      { border-left-color: var(--ui-success); }
.ops-bar--warn    { border-left-color: var(--ui-warning); }
.ops-bar--down    { border-left-color: var(--ui-danger); }
.ops-bar--unknown { border-left-color: var(--ui-border); }

.bar-section { display: flex; align-items: center; gap: var(--space-2); min-width: 0; }
.bar-section--status { flex-shrink: 0; min-width: 220px; }
.bar-section--meta {
  flex: 1;
  gap: var(--space-4);
  color: var(--ui-fg-3);
  font-size: var(--fs-xs);
  overflow: hidden;
  min-width: 0;
}
.bar-section--actions { flex-shrink: 0; gap: var(--space-2); margin-left: auto; }

.bar-dot {
  width: 8px; height: 8px; border-radius: 50%;
  background: var(--ui-fg-4);
  flex-shrink: 0;
}
.ops-bar--ok .bar-dot {
  background: var(--ui-success);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--ui-success) 25%, transparent);
  animation: bar-pulse 2s ease-in-out infinite;
}
.ops-bar--warn .bar-dot { background: var(--ui-warning); box-shadow: 0 0 0 3px color-mix(in srgb, var(--ui-warning) 25%, transparent); }
.ops-bar--down .bar-dot { background: var(--ui-danger); }

@keyframes bar-pulse {
  0%, 100% { box-shadow: 0 0 0 3px color-mix(in srgb, var(--ui-success) 25%, transparent); }
  50%      { box-shadow: 0 0 0 6px color-mix(in srgb, var(--ui-success) 10%, transparent); }
}

.bar-name {
  font-family: var(--font-mono);
  font-weight: var(--fw-semibold);
  font-size: var(--fs-sm);
  color: var(--ui-fg);
  max-width: 180px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.bar-state {
  font-size: var(--fs-xs);
  color: var(--ui-fg-3);
  border-left: 1px solid var(--ui-border);
  margin-left: var(--space-1);
  padding-left: var(--space-2);
}

.bar-kv {
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
  min-width: 0;
  max-width: 100%;
}
.bar-kv--ports { max-width: 300px; }
.bar-k {
  font-size: var(--fs-xs);
  color: var(--ui-fg-3);
  text-transform: uppercase;
  letter-spacing: .3px;
  flex-shrink: 0;
}
.bar-v {
  font-size: var(--fs-xs);
  color: var(--ui-fg);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}
code.bar-v {
  background: var(--ui-bg-1);
  padding: 1px 5px;
  border-radius: var(--radius-sm);
  font-family: var(--font-mono);
}
.bar-kv--missing { color: var(--ui-danger-fg); font-size: var(--fs-xs); }

@media (max-width: 900px) {
  .bar-section--meta { flex-basis: 100%; order: 3; padding-top: var(--space-1); border-top: 1px dashed var(--ui-border); margin-top: var(--space-1); }
  .bar-section--actions { margin-left: 0; }
}
</style>
