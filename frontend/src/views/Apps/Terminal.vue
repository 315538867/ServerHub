<template>
  <div class="terminal-page">
    <div class="tab-bar">
      <div
        v-for="tab in tabs"
        :key="tab.id"
        class="tab-item"
        :class="{ active: activeTabId === tab.id }"
        @click="switchTab(tab.id)"
      >
        <span class="tab-dot" :class="tab.status" />
        <span class="tab-label">终端 {{ tab.id }}</span>
        <close-icon class="tab-close" @click.stop="closeTab(tab.id)" />
      </div>
      <t-button class="tab-add" shape="circle" size="small" variant="text" theme="default" @click="addTab">
        <template #icon><add-icon /></template>
      </t-button>
    </div>
    <div v-if="searchVisible" class="search-bar">
      <t-input v-model="searchQuery" placeholder="搜索…" size="small" style="width:200px"
        @keydown.enter="searchNext" @keydown.escape="closeSearch" ref="searchInputEl" />
      <t-checkbox v-model="searchCaseSensitive">区分大小写</t-checkbox>
      <t-checkbox v-model="searchRegex">正则</t-checkbox>
      <t-button size="small" variant="outline" @click="searchPrev">↑</t-button>
      <t-button size="small" variant="outline" @click="searchNext">↓</t-button>
      <t-button size="small" variant="text" @click="closeSearch"><template #icon><close-icon /></template></t-button>
    </div>
    <div class="terminals-wrapper">
      <div
        v-for="tab in tabs"
        :key="tab.id"
        :ref="el => { if (el) terminalEls[tab.id] = el as HTMLDivElement }"
        class="terminal-container"
        :style="{ display: activeTabId === tab.id ? 'block' : 'none' }"
      />
      <div v-if="tabs.length === 0" class="terminal-empty">
        <t-empty description="连接中…" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onMounted, onBeforeUnmount } from 'vue'
import { useRoute } from 'vue-router'
import { AddIcon, CloseIcon } from 'tdesign-icons-vue-next'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { SearchAddon } from '@xterm/addon-search'
import '@xterm/xterm/css/xterm.css'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'

const route = useRoute()
const authStore = useAuthStore()
const appStore = useAppStore()

const appId = computed(() => Number(route.params.appId))
const serverId = computed(() => appStore.getById(appId.value)?.server_id ?? 0)

type TabStatus = 'connecting' | 'connected' | 'disconnected'
interface TermTab { id: number; status: TabStatus }

let nextId = 1
const tabs = ref<TermTab[]>([])
const activeTabId = ref<number | null>(null)
const terminalEls: Record<number, HTMLDivElement> = {}
const terms: Record<number, Terminal> = {}
const fitAddons: Record<number, FitAddon> = {}
const searchAddons: Record<number, SearchAddon> = {}
const wss: Record<number, WebSocket> = {}
const resizeObs: Record<number, ResizeObserver> = {}

const searchVisible = ref(false)
const searchQuery = ref('')
const searchCaseSensitive = ref(false)
const searchRegex = ref(false)
const searchInputEl = ref()

onMounted(async () => {
  if (!appStore.apps.length) await appStore.fetch()
  await createTab()
  window.addEventListener('keydown', handleGlobalKey)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleGlobalKey)
  tabs.value.forEach(t => destroyTab(t.id))
})

function handleGlobalKey(e: KeyboardEvent) {
  if ((e.ctrlKey || e.metaKey) && e.key === 'f') {
    if (activeTabId.value !== null) { e.preventDefault(); openSearch() }
  }
}

async function addTab() { await createTab() }

async function createTab() {
  const id = nextId++
  tabs.value.push({ id, status: 'connecting' })
  activeTabId.value = id
  await nextTick()
  initTerm(id)
}

function initTerm(id: number) {
  const el = terminalEls[id]
  if (!el) return
  const term = new Terminal({
    cursorBlink: true, fontSize: 14,
    fontFamily: '"Cascadia Code", "JetBrains Mono", Menlo, Monaco, monospace',
    theme: { background: '#1a2332', foreground: '#e0e0e0', cursor: '#0052d9' },
    scrollback: 5000, convertEol: true,
  })
  const fit = new FitAddon()
  const search = new SearchAddon()
  term.loadAddon(fit); term.loadAddon(search)
  term.open(el); fit.fit()
  terms[id] = term; fitAddons[id] = fit; searchAddons[id] = search
  const ro = new ResizeObserver(() => {
    fit.fit()
    const ws = wss[id]
    if (ws?.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }))
  })
  ro.observe(el); resizeObs[id] = ro
  connectWs(id)
}

function connectWs(id: number) {
  const tab = tabs.value.find(t => t.id === id)
  if (!tab) return
  tab.status = 'connecting'
  const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
  const url = `${protocol}//${location.host}/panel/api/v1/servers/${serverId.value}/terminal?token=${authStore.token}`
  const ws = new WebSocket(url)
  ws.binaryType = 'arraybuffer'
  wss[id] = ws
  const term = terms[id]
  if (!term) return
  ws.onopen = () => {
    if (tab) tab.status = 'connected'
    term.clear(); fitAddons[id]?.fit()
    ws.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }))
    term.onData(data => { if (ws.readyState === WebSocket.OPEN) ws.send(new TextEncoder().encode(data)) })
  }
  ws.onmessage = e => {
    if (e.data instanceof ArrayBuffer) term.write(new Uint8Array(e.data))
    else term.writeln('\x1b[31m' + e.data + '\x1b[0m')
  }
  ws.onclose = () => {
    if (tab) tab.status = 'disconnected'
    term.writeln('\r\n\x1b[33m[连接已断开] 按 Enter 重连\x1b[0m')
    const disp = term.onData(key => { if (key === '\r') { disp.dispose(); term.clear(); connectWs(id) } })
  }
  ws.onerror = () => { if (tab) tab.status = 'disconnected'; term.writeln('\r\n\x1b[31m[连接错误]\x1b[0m') }
}

function switchTab(id: number) {
  activeTabId.value = id
  nextTick(() => fitAddons[id]?.fit())
}

function closeTab(id: number) {
  destroyTab(id)
  const idx = tabs.value.findIndex(t => t.id === id)
  tabs.value.splice(idx, 1)
  if (activeTabId.value === id) activeTabId.value = tabs.value[Math.max(0, idx - 1)]?.id ?? null
}

function destroyTab(id: number) {
  wss[id]?.close(); delete wss[id]
  resizeObs[id]?.disconnect(); delete resizeObs[id]
  terms[id]?.dispose(); delete terms[id]
  delete fitAddons[id]; delete searchAddons[id]; delete terminalEls[id]
}

function openSearch() { searchVisible.value = true; nextTick(() => searchInputEl.value?.focus()) }
function closeSearch() { searchVisible.value = false; searchQuery.value = '' }
function searchNext() {
  if (!activeTabId.value) return
  searchAddons[activeTabId.value]?.findNext(searchQuery.value, { caseSensitive: searchCaseSensitive.value, regex: searchRegex.value })
}
function searchPrev() {
  if (!activeTabId.value) return
  searchAddons[activeTabId.value]?.findPrevious(searchQuery.value, { caseSensitive: searchCaseSensitive.value, regex: searchRegex.value })
}
</script>

<style scoped>
.terminal-page {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: #1a2332;
}

.tab-bar {
  display: flex;
  align-items: center;
  gap: var(--sh-space-xs);
  padding: var(--sh-space-xs) var(--sh-space-sm);
  background: #152030;
  border-bottom: 1px solid #243447;
  flex-shrink: 0;
  overflow-x: auto;
}

.tab-item {
  display: flex;
  align-items: center;
  gap: var(--sh-space-sm);
  padding: var(--sh-space-xs) var(--sh-space-sm);
  border-radius: 4px;
  cursor: pointer;
  color: #8a94a6;
  font-size: 13px;
  white-space: nowrap;
  transition: background 0.15s;
}

.tab-item:hover { background: #1e2d3d; }
.tab-item.active { background: #1a2332; color: #e0e0e0; }

.tab-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  flex-shrink: 0;
}

.tab-dot.connecting { background: var(--sh-orange); }
.tab-dot.connected { background: var(--sh-green); }
.tab-dot.disconnected { background: var(--sh-red); }

.tab-close {
  opacity: 0.4;
  font-size: 12px;
  transition: opacity 0.15s;
  cursor: pointer;
}

.tab-close:hover { opacity: 1; }

.tab-add {
  margin-left: var(--sh-space-xs);
  flex-shrink: 0;
}

.search-bar {
  display: flex;
  align-items: center;
  gap: var(--sh-space-sm);
  padding: var(--sh-space-sm) var(--sh-space-md);
  background: #152030;
  border-bottom: 1px solid #243447;
  flex-shrink: 0;
}

.terminals-wrapper {
  flex: 1;
  overflow: hidden;
  position: relative;
}

.terminal-container {
  width: 100%;
  height: 100%;
  padding: var(--sh-space-xs);
}

.terminal-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
}

:deep(.xterm) { height: 100%; }
:deep(.xterm-viewport) { background-color: #1a2332 !important; }
</style>
