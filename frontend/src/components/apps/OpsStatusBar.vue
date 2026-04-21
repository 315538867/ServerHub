<template>
  <div v-if="app?.container_name" class="ops-bar" :class="barStateClass">
    <!-- 左：状态 -->
    <div class="bar-section bar-section--status">
      <span class="bar-dot" />
      <span class="bar-name" :title="app.container_name">{{ app.container_name }}</span>
      <span class="bar-state">{{ stateText }}</span>
    </div>

    <!-- 中：元信息 -->
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

    <!-- 右：快捷操作 -->
    <div class="bar-section bar-section--actions">
      <t-button
        v-if="container && container.state !== 'running'"
        size="small" theme="success" variant="outline"
        :loading="acting === 'start'"
        @click="doAction('start')"
      >启动</t-button>
      <t-button
        v-if="container?.state === 'running'"
        size="small" theme="warning" variant="outline"
        :loading="acting === 'stop'"
        @click="doAction('stop')"
      >停止</t-button>
      <t-button
        v-if="container"
        size="small" variant="outline"
        :loading="acting === 'restart'"
        @click="doAction('restart')"
      >重启</t-button>
      <t-button size="small" variant="text" :loading="loading" @click="refresh" title="刷新">
        <template #icon><refresh-icon /></template>
      </t-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted, onBeforeUnmount, watch } from 'vue'
import { RefreshIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'
import { useAppStore } from '@/stores/app'
import { getContainers, containerAction } from '@/api/docker'
import type { ContainerItem } from '@/api/docker'

const props = defineProps<{ appId: number }>()
const emit = defineEmits<{ (e: 'changed'): void }>()

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
    MessagePlugin.success(({ start: '已启动', stop: '已停止', restart: '已重启' } as Record<string, string>)[action])
    await refresh()
    emit('changed')
  } catch { MessagePlugin.error('操作失败') }
  finally { acting.value = '' }
}

function startPoll() { stopPoll(); timer = setInterval(() => refresh(true), 10000) }
function stopPoll() { if (timer) { clearInterval(timer); timer = null } }

watch(() => props.appId, () => { refresh(); })
onMounted(() => { refresh(); startPoll() })
onBeforeUnmount(stopPoll)
</script>

<style scoped>
.ops-bar {
  display: flex;
  align-items: center;
  gap: var(--ui-space-6);
  padding: var(--ui-space-2) var(--ui-space-4);
  background: var(--ui-bg-surface);
  border: 1px solid var(--ui-border);
  border-radius: 8px;
  margin: 0 0 var(--ui-space-2);
  border-left: 3px solid #999;
  flex-wrap: wrap;
}
.ops-bar--ok      { border-left-color: #67c23a; }
.ops-bar--warn    { border-left-color: #e6a23c; }
.ops-bar--down    { border-left-color: #e34d59; }
.ops-bar--unknown { border-left-color: #999; }

.bar-section { display: flex; align-items: center; gap: var(--ui-space-2); min-width: 0; }
.bar-section--status { flex-shrink: 0; min-width: 220px; }
.bar-section--meta {
  flex: 1;
  gap: var(--ui-space-4);
  color: var(--ui-fg-3);
  font-size: 12.5px;
  overflow: hidden;
  min-width: 0;
}
.bar-section--actions { flex-shrink: 0; gap: var(--ui-space-2); margin-left: auto; }

.bar-dot {
  width: 8px; height: 8px; border-radius: 50%;
  background: #999;
  flex-shrink: 0;
}
.ops-bar--ok .bar-dot {
  background: #67c23a;
  box-shadow: 0 0 0 3px color-mix(in srgb, #67c23a 25%, transparent);
  animation: bar-pulse 2s ease-in-out infinite;
}
.ops-bar--warn .bar-dot { background: #e6a23c; box-shadow: 0 0 0 3px color-mix(in srgb, #e6a23c 25%, transparent); }
.ops-bar--down .bar-dot { background: #e34d59; }

@keyframes bar-pulse {
  0%, 100% { box-shadow: 0 0 0 3px color-mix(in srgb, #67c23a 25%, transparent); }
  50%      { box-shadow: 0 0 0 6px color-mix(in srgb, #67c23a 10%, transparent); }
}

.bar-name {
  font-family: var(--ui-font-mono, ui-monospace, SFMono-Regular, monospace);
  font-weight: 600;
  font-size: 13px;
  color: var(--ui-fg);
  max-width: 180px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.bar-state {
  font-size: 12px;
  color: var(--ui-fg-3);
  padding-left: var(--ui-space-1);
  border-left: 1px solid var(--ui-border);
  margin-left: var(--ui-space-1);
  padding-left: var(--ui-space-2);
}

.bar-kv {
  display: inline-flex;
  align-items: center;
  gap: var(--ui-space-2);
  min-width: 0;
  max-width: 100%;
}
.bar-kv--ports { max-width: 300px; }
.bar-k {
  font-size: 11px;
  color: var(--ui-fg-3);
  text-transform: uppercase;
  letter-spacing: .3px;
  flex-shrink: 0;
}
.bar-v {
  font-size: 12px;
  color: var(--ui-fg);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}
code.bar-v {
  background: var(--ui-bg-subtle, rgba(0,0,0,.04));
  padding: 1px 5px;
  border-radius: 3px;
  font-family: var(--ui-font-mono, ui-monospace, SFMono-Regular, monospace);
}
.bar-kv--missing { color: #e34d59; font-size: 12px; }

@media (max-width: 900px) {
  .bar-section--meta { flex-basis: 100%; order: 3; padding-top: var(--ui-space-1); border-top: 1px dashed var(--ui-border); margin-top: var(--ui-space-1); }
  .bar-section--actions { margin-left: 0; }
}
</style>
