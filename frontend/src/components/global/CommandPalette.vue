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

// ── 全局快捷键 ⌘K / Ctrl+K ──────────────────────────────────────────────────
function onGlobalKey(e: KeyboardEvent) {
  const isMod = e.metaKey || e.ctrlKey
  if (isMod && e.key.toLowerCase() === 'k') {
    e.preventDefault()
    toggle()
    return
  }
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
  background: rgba(0, 0, 0, 0.42);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding-top: 14vh;
}
.cp-box {
  width: min(640px, 92vw);
  background: var(--sh-card-bg, #fff);
  border: 1px solid var(--sh-border);
  border-radius: 14px;
  box-shadow: 0 30px 80px -10px rgba(0, 0, 0, 0.35);
  overflow: hidden;
  display: flex;
  flex-direction: column;
  max-height: 70vh;
}

.cp-input-wrap {
  display: flex;
  align-items: center;
  gap: var(--sh-space-sm);
  padding: var(--sh-space-md) var(--sh-space-lg);
  border-bottom: 1px solid var(--sh-border);
}
.cp-input-icon {
  font-size: 18px;
  color: var(--sh-text-secondary);
  font-weight: 600;
}
.cp-input {
  flex: 1;
  border: 0;
  outline: 0;
  background: transparent;
  font-size: 15px;
  color: var(--sh-text-primary);
}
.cp-input::placeholder { color: var(--sh-text-secondary); }
.cp-input-esc {
  font-size: 11px;
  padding: 1px 8px;
  border: 1px solid var(--sh-border);
  border-radius: 4px;
  color: var(--sh-text-secondary);
  background: var(--sh-bg);
}

.cp-list {
  flex: 1;
  overflow-y: auto;
  padding: var(--sh-space-sm) 0;
  min-height: 80px;
}
.cp-group { padding: var(--sh-space-xs) 0 var(--sh-space-sm); }
.cp-group-label {
  font-size: 11px;
  letter-spacing: 0.5px;
  text-transform: uppercase;
  color: var(--sh-text-secondary);
  padding: var(--sh-space-xs) var(--sh-space-lg);
}
.cp-item {
  display: flex;
  align-items: center;
  gap: var(--sh-space-md);
  padding: var(--sh-space-sm) var(--sh-space-lg);
  cursor: pointer;
  transition: background 0.08s;
}
.cp-item--active {
  background: color-mix(in srgb, var(--sh-blue) 12%, transparent);
}
.cp-item-icon {
  font-size: 16px;
  width: 20px;
  text-align: center;
  flex-shrink: 0;
}
.cp-item-text {
  flex: 1;
  min-width: 0;
}
.cp-item-title {
  font-size: 14px;
  color: var(--sh-text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.cp-item-title :deep(mark) {
  background: color-mix(in srgb, var(--sh-blue) 30%, transparent);
  color: inherit;
  padding: 0 var(--sh-space-xs);
  border-radius: 2px;
}
.cp-item-sub {
  font-size: 12px;
  color: var(--sh-text-secondary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  margin-top: var(--sh-space-xs);
}
.cp-item-sub :deep(mark) {
  background: color-mix(in srgb, var(--sh-blue) 22%, transparent);
  color: inherit;
}
.cp-item-shortcut {
  display: inline-flex;
  gap: var(--sh-space-xs);
}
.cp-item-shortcut kbd {
  font-size: 11px;
  padding: 1px 6px;
  border: 1px solid var(--sh-border);
  border-radius: 4px;
  background: var(--sh-bg);
  color: var(--sh-text-secondary);
}
.cp-item-hint {
  font-size: 12px;
  color: var(--sh-text-secondary);
  opacity: 0;
  transition: opacity 0.1s;
}
.cp-item--active .cp-item-hint { opacity: 1; }

.cp-empty {
  text-align: center;
  padding: var(--sh-space-xl) 0;
  color: var(--sh-text-secondary);
}
.cp-empty-icon { font-size: 28px; margin-bottom: var(--sh-space-sm); opacity: 0.6; }
.cp-empty-text { font-size: 13px; }

.cp-footer {
  display: flex;
  align-items: center;
  gap: var(--sh-space-md);
  padding: var(--sh-space-sm) var(--sh-space-lg);
  border-top: 1px solid var(--sh-border);
  background: color-mix(in srgb, var(--sh-text-primary) 3%, transparent);
  font-size: 11px;
  color: var(--sh-text-secondary);
}
.cp-footer kbd {
  font-size: 10px;
  padding: 1px 5px;
  border: 1px solid var(--sh-border);
  border-radius: 3px;
  background: var(--sh-card-bg);
  margin-right: var(--sh-space-xs);
}
.cp-footer-spacer { flex: 1; }

.cp-fade-enter-active,
.cp-fade-leave-active {
  transition: opacity 0.14s;
}
.cp-fade-enter-from,
.cp-fade-leave-to { opacity: 0; }
.cp-fade-enter-active .cp-box,
.cp-fade-leave-active .cp-box {
  transition: transform 0.14s, opacity 0.14s;
}
.cp-fade-enter-from .cp-box,
.cp-fade-leave-to .cp-box { transform: translateY(-8px); opacity: 0; }
</style>
