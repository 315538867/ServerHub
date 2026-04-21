<template>
  <transition name="cp-fade">
    <div v-if="open" class="cp-overlay" @click.self="close" @keydown.esc="close">
      <div class="cp-box" role="dialog" aria-label="命令面板">
        <div class="cp-input-wrap">
          <span class="cp-input-icon">⌘</span>
          <input
            ref="inputRef"
            v-model="query"
            type="text"
            class="cp-input"
            placeholder="搜索应用、服务器、页面，或输入操作…"
            @keydown.down.prevent="moveCursor(1)"
            @keydown.up.prevent="moveCursor(-1)"
            @keydown.enter.prevent="executeCurrent"
            @keydown.esc="close"
          />
          <kbd class="cp-input-esc">esc</kbd>
        </div>

        <div ref="listRef" class="cp-list">
          <template v-if="grouped.length">
            <div v-for="group in grouped" :key="group.label" class="cp-group">
              <div class="cp-group-label">{{ group.label }}</div>
              <div
                v-for="(item, gi) in group.items"
                :key="item.id"
                :class="['cp-item', { 'cp-item--active': item._idx === cursor }]"
                @mouseenter="cursor = item._idx"
                @click="execute(item)"
                :data-idx="item._idx"
              >
                <span class="cp-item-icon">{{ item.icon }}</span>
                <div class="cp-item-text">
                  <div class="cp-item-title" v-html="highlight(item.title)" />
                  <div v-if="item.subtitle" class="cp-item-sub" v-html="highlight(item.subtitle)" />
                </div>
                <span v-if="item.shortcut" class="cp-item-shortcut">
                  <kbd v-for="k in item.shortcut.split('+')" :key="k">{{ k }}</kbd>
                </span>
                <span v-else class="cp-item-hint">↵</span>
              </div>
            </div>
          </template>

          <div v-else class="cp-empty">
            <div class="cp-empty-icon">🔍</div>
            <div class="cp-empty-text">未找到匹配项</div>
          </div>
        </div>

        <div class="cp-footer">
          <span><kbd>↑</kbd><kbd>↓</kbd> 移动</span>
          <span><kbd>↵</kbd> 选中</span>
          <span><kbd>esc</kbd> 关闭</span>
          <span class="cp-footer-spacer" />
          <span>{{ flatItems.length }} 项</span>
        </div>
      </div>
    </div>
  </transition>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onBeforeUnmount } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAppStore } from '@/stores/app'
import { useServerStore } from '@/stores/server'

const router = useRouter()
const route = useRoute()
const appStore = useAppStore()
const serverStore = useServerStore()

const open = ref(false)
const query = ref('')
const cursor = ref(0)
const inputRef = ref<HTMLInputElement | null>(null)
const listRef = ref<HTMLDivElement | null>(null)

interface CmdItem {
  id: string
  group: string
  icon: string
  title: string
  subtitle?: string
  shortcut?: string
  action: () => void
  keywords?: string
  _idx?: number
}

// ── 全局快捷键 ⌘D / ⌘L（⌘K 由 MainLayout 处理） ─────────────────────────────
function onGlobalKey(e: KeyboardEvent) {
  const isMod = e.metaKey || e.ctrlKey
  // ⌘D 部署当前应用
  if (isMod && e.key.toLowerCase() === 'd' && currentAppId.value) {
    e.preventDefault()
    router.push(`/apps/${currentAppId.value}/deploy`)
    return
  }
  // ⌘L 日志
  if (isMod && e.key.toLowerCase() === 'l' && currentAppId.value) {
    e.preventDefault()
    router.push(`/apps/${currentAppId.value}/ops/logs`)
    return
  }
}

const currentAppId = computed(() => {
  const m = /^\/apps\/(\d+)/.exec(route.path)
  return m ? m[1] : ''
})

function toggle() { open.value ? close() : openPalette() }
function openPalette() {
  open.value = true
  query.value = ''
  cursor.value = 0
  nextTick(() => inputRef.value?.focus())
}
function close() { open.value = false }

// ── 候选项构造 ──────────────────────────────────────────────────────────────
const navItems = computed<CmdItem[]>(() => [
  { id: 'nav:dashboard',  group: '页面', icon: '🏠', title: '工作台',     action: () => router.push('/dashboard') },
  { id: 'nav:apps',       group: '页面', icon: '📋', title: '应用列表',   action: () => router.push('/apps') },
  { id: 'nav:create',     group: '页面', icon: '➕', title: '新建应用',   action: () => router.push('/apps/create') },
  { id: 'nav:servers',    group: '页面', icon: '🖥️', title: '服务器列表', action: () => router.push('/servers') },
  { id: 'nav:deploy',     group: '页面', icon: '🚀', title: '全局部署',   action: () => router.push('/deploy') },
  { id: 'nav:database',   group: '页面', icon: '🗄️', title: '全局数据库', action: () => router.push('/database') },
  { id: 'nav:notif',      group: '页面', icon: '🔔', title: '通知中心',   action: () => router.push('/notifications') },
  { id: 'nav:settings',   group: '页面', icon: '⚙️', title: '设置',       action: () => router.push('/settings') },
])

const appItems = computed<CmdItem[]>(() =>
  appStore.apps.flatMap(a => [
    {
      id: `app:${a.id}`,
      group: '应用',
      icon: a.status === 'online' ? '🟢' : a.status === 'error' ? '⚠️' : '⚪',
      title: a.name,
      subtitle: [a.domain, a.container_name, a.description].filter(Boolean).join(' · '),
      keywords: `${a.name} ${a.domain || ''} ${a.container_name || ''} ${a.description || ''} ${a.site_name || ''}`,
      action: () => router.push(`/apps/${a.id}/overview`),
    },
    {
      id: `app-deploy:${a.id}`,
      group: '应用',
      icon: '🚀',
      title: `部署 ${a.name}`,
      subtitle: '直接进入部署驾驶舱',
      keywords: `deploy 部署 ${a.name}`,
      action: () => router.push(`/apps/${a.id}/deploy`),
    },
  ])
)

const serverItems = computed<CmdItem[]>(() =>
  serverStore.servers.map(s => ({
    id: `srv:${s.id}`,
    group: '服务器',
    icon: s.status === 'online' ? '🟢' : '🔴',
    title: s.name,
    subtitle: `${s.host}:${s.port} · ${s.username}`,
    keywords: `${s.name} ${s.host}`,
    action: () => router.push(`/servers/${s.id}/overview`),
  }))
)

// 上下文操作（仅在某应用页内）
const contextItems = computed<CmdItem[]>(() => {
  if (!currentAppId.value) return []
  const id = currentAppId.value
  const app = appStore.getById(Number(id))
  const name = app?.name || '当前应用'
  return [
    { id: 'ctx:deploy',  group: '操作（当前应用）', icon: '▶',  title: `部署 ${name}`,  shortcut: '⌘+D', action: () => router.push(`/apps/${id}/deploy`) },
    { id: 'ctx:logs',    group: '操作（当前应用）', icon: '📜', title: `查看日志`,      shortcut: '⌘+L', action: () => router.push(`/apps/${id}/ops/logs`) },
    { id: 'ctx:term',    group: '操作（当前应用）', icon: '💻', title: `打开终端`,                       action: () => router.push(`/apps/${id}/ops/terminal`) },
    { id: 'ctx:metrics', group: '操作（当前应用）', icon: '📊', title: `实时指标 / 总览`,                 action: () => router.push(`/apps/${id}/overview`) },
  ]
})

const allItems = computed(() => [
  ...contextItems.value,
  ...navItems.value,
  ...appItems.value,
  ...serverItems.value,
])

// 模糊匹配：拆词 + 全部命中
function fuzzy(item: CmdItem, kw: string): boolean {
  if (!kw) return true
  const hay = `${item.title} ${item.subtitle || ''} ${item.keywords || ''}`.toLowerCase()
  return kw.toLowerCase().split(/\s+/).every(t => hay.includes(t))
}

const flatItems = computed(() => {
  const list = allItems.value.filter(i => fuzzy(i, query.value))
  return list.map((it, i) => ({ ...it, _idx: i }))
})

const grouped = computed(() => {
  const map = new Map<string, typeof flatItems.value>()
  for (const it of flatItems.value) {
    if (!map.has(it.group)) map.set(it.group, [])
    map.get(it.group)!.push(it)
  }
  return Array.from(map.entries()).map(([label, items]) => ({ label, items }))
})

watch(query, () => { cursor.value = 0 })
watch(cursor, () => nextTick(scrollIntoView))

function moveCursor(d: number) {
  const n = flatItems.value.length
  if (n === 0) return
  cursor.value = (cursor.value + d + n) % n
}

function scrollIntoView() {
  const el = listRef.value?.querySelector(`[data-idx="${cursor.value}"]`) as HTMLElement | null
  el?.scrollIntoView({ block: 'nearest' })
}

function executeCurrent() {
  const it = flatItems.value[cursor.value]
  if (it) execute(it)
}
function execute(it: CmdItem) {
  close()
  it.action()
}

function highlight(text: string): string {
  const kw = query.value.trim()
  if (!kw) return escape(text)
  const parts = kw.split(/\s+/).filter(Boolean)
  let out = escape(text)
  for (const p of parts) {
    const re = new RegExp(`(${escapeRe(p)})`, 'gi')
    out = out.replace(re, '<mark>$1</mark>')
  }
  return out
}
function escape(s: string) { return s.replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]!)) }
function escapeRe(s: string) { return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') }

onMounted(() => window.addEventListener('keydown', onGlobalKey))
onBeforeUnmount(() => window.removeEventListener('keydown', onGlobalKey))

defineExpose({ open: openPalette, close })
</script>

<style scoped>
.cp-overlay {
  position: fixed;
  inset: 0;
  z-index: 9999;
  background: var(--ui-bg-overlay);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding-top: 12vh;
}
.cp-box {
  width: min(680px, 92vw);
  background: var(--ui-bg-elevated);
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-xl);
  box-shadow: var(--ui-shadow-xl), 0 0 0 1px var(--ui-border-subtle);
  overflow: hidden;
  display: flex;
  flex-direction: column;
  max-height: 70vh;
  position: relative;
}
.cp-box::before {
  content: '';
  position: absolute;
  inset: 0;
  pointer-events: none;
  border-radius: inherit;
  background: radial-gradient(120% 100% at 0% 0%, var(--ui-brand-soft) 0%, transparent 60%);
  opacity: .55;
}

.cp-input-wrap {
  position: relative;
  display: flex;
  align-items: center;
  gap: var(--ui-space-3);
  padding: var(--ui-space-3) var(--ui-space-5);
  border-bottom: 1px solid var(--ui-border-subtle);
}
.cp-input-icon {
  font-size: 16px;
  color: var(--ui-brand);
  font-weight: 700;
  filter: drop-shadow(0 0 6px var(--ui-brand-soft));
}
.cp-input {
  flex: 1;
  border: 0;
  outline: 0;
  background: transparent;
  font-size: var(--ui-fs-md);
  color: var(--ui-fg);
  font-weight: var(--ui-fw-medium);
}
.cp-input::placeholder { color: var(--ui-fg-placeholder); font-weight: 400; }
.cp-input-esc {
  font-size: var(--ui-fs-2xs);
  padding: 2px 8px;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-sm);
  color: var(--ui-fg-3);
  background: var(--ui-bg-subtle);
  font-family: var(--ui-font-mono);
}

.cp-list {
  position: relative;
  flex: 1;
  overflow-y: auto;
  padding: var(--ui-space-2) 0;
  min-height: 90px;
}
.cp-group { padding: var(--ui-space-1) 0 var(--ui-space-2); animation: ui-fade-in var(--ui-dur-base) var(--ui-ease-standard); }
.cp-group-label {
  font-size: var(--ui-fs-2xs);
  letter-spacing: .1em;
  text-transform: uppercase;
  color: var(--ui-fg-4);
  font-weight: var(--ui-fw-semibold);
  padding: var(--ui-space-2) var(--ui-space-5) var(--ui-space-1);
}
.cp-item {
  display: flex;
  align-items: center;
  gap: var(--ui-space-3);
  padding: 8px var(--ui-space-5);
  margin: 0 var(--ui-space-2);
  border-radius: var(--ui-radius-md);
  cursor: pointer;
  transition: background var(--ui-dur-fast) var(--ui-ease-standard),
              transform var(--ui-dur-fast) var(--ui-ease-standard);
  position: relative;
}
.cp-item:hover { background: var(--ui-bg-hover); }
.cp-item--active {
  background: var(--ui-brand-soft);
  transform: translateX(2px);
}
.cp-item--active::before {
  content: '';
  position: absolute;
  left: -2px; top: 8px; bottom: 8px;
  width: 3px;
  background: var(--ui-brand);
  border-radius: 0 3px 3px 0;
  animation: ui-slide-right var(--ui-dur-base) var(--ui-ease-standard);
}
.cp-item-icon {
  font-size: 16px;
  width: 24px; height: 24px;
  display: grid; place-items: center;
  text-align: center;
  flex-shrink: 0;
  background: var(--ui-bg-subtle);
  border-radius: var(--ui-radius-sm);
  border: 1px solid var(--ui-border-subtle);
}
.cp-item--active .cp-item-icon {
  background: var(--ui-bg-surface);
  border-color: var(--ui-brand);
  color: var(--ui-brand);
}
.cp-item-text { flex: 1; min-width: 0; }
.cp-item-title {
  font-size: var(--ui-fs-sm);
  color: var(--ui-fg);
  font-weight: var(--ui-fw-medium);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.cp-item-title :deep(mark) {
  background: linear-gradient(180deg, transparent 60%, var(--ui-brand-soft-2) 60%);
  color: var(--ui-brand);
  font-weight: var(--ui-fw-semibold);
  padding: 0 1px;
  border-radius: 2px;
}
.cp-item-sub {
  font-size: var(--ui-fs-xs);
  color: var(--ui-fg-3);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  margin-top: 2px;
}
.cp-item-sub :deep(mark) {
  background: var(--ui-brand-soft);
  color: var(--ui-brand);
  padding: 0 2px;
  border-radius: 2px;
}
.cp-item-shortcut { display: inline-flex; gap: 4px; }
.cp-item-shortcut kbd {
  font-size: var(--ui-fs-2xs);
  padding: 2px 6px;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-sm);
  background: var(--ui-bg-surface);
  color: var(--ui-fg-2);
  font-family: var(--ui-font-mono);
  font-weight: 600;
  min-width: 18px;
  text-align: center;
  box-shadow: 0 1px 0 var(--ui-border);
}
.cp-item-hint {
  font-size: var(--ui-fs-sm);
  color: var(--ui-brand);
  opacity: 0;
  transition: opacity var(--ui-dur-fast), transform var(--ui-dur-fast);
}
.cp-item--active .cp-item-hint {
  opacity: 1;
  animation: cp-arrow .8s var(--ui-ease-standard) infinite;
}
@keyframes cp-arrow {
  0%, 100% { transform: translateX(0); }
  50%      { transform: translateX(3px); }
}

.cp-empty {
  text-align: center;
  padding: var(--ui-space-8) 0;
  color: var(--ui-fg-3);
  animation: ui-fade-in var(--ui-dur-base) var(--ui-ease-standard);
}
.cp-empty-icon {
  font-size: 32px;
  margin-bottom: var(--ui-space-2);
  opacity: 0.7;
  animation: ui-pulse 2s var(--ui-ease-standard) infinite;
}
.cp-empty-text { font-size: var(--ui-fs-sm); font-weight: var(--ui-fw-medium); }

.cp-footer {
  display: flex;
  align-items: center;
  gap: var(--ui-space-4);
  padding: 8px var(--ui-space-5);
  border-top: 1px solid var(--ui-border-subtle);
  background: var(--ui-bg-subtle);
  font-size: var(--ui-fs-2xs);
  color: var(--ui-fg-3);
}
.cp-footer kbd {
  font-size: var(--ui-fs-2xs);
  padding: 1px 5px;
  border: 1px solid var(--ui-border);
  border-radius: 3px;
  background: var(--ui-bg-surface);
  margin-right: 4px;
  font-family: var(--ui-font-mono);
  color: var(--ui-fg-2);
}
.cp-footer-spacer { flex: 1; }

.cp-fade-enter-active,
.cp-fade-leave-active {
  transition: opacity var(--ui-dur-base) var(--ui-ease-standard);
}
.cp-fade-enter-from,
.cp-fade-leave-to { opacity: 0; }
.cp-fade-enter-active .cp-box {
  animation: cp-pop var(--ui-dur-base) var(--ui-ease-spring);
}
.cp-fade-leave-active .cp-box {
  transition: transform var(--ui-dur-fast), opacity var(--ui-dur-fast);
}
.cp-fade-leave-to .cp-box { transform: translateY(-12px) scale(.98); opacity: 0; }

@keyframes cp-pop {
  0%   { transform: translateY(-16px) scale(.96); opacity: 0; }
  60%  { transform: translateY(2px)   scale(1.01); opacity: 1; }
  100% { transform: translateY(0)     scale(1); }
}

@media (prefers-reduced-motion: reduce) {
  .cp-fade-enter-active .cp-box,
  .cp-item--active .cp-item-hint,
  .cp-empty-icon { animation: none; }
}
</style>
