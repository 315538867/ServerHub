<template>
  <div class="term-page">
    <div class="term-tabs">
      <div
        v-for="tab in tabs"
        :key="tab.id"
        class="term-tab"
        :class="{ 'is-active': activeTabId === tab.id }"
        @click="switchTab(tab.id)"
      >
        <span class="term-tab__dot" :class="`is-${tab.status}`" />
        <span class="term-tab__label">终端 {{ tab.id }}</span>
        <X :size="12" class="term-tab__close" @click.stop="closeTab(tab.id)" />
      </div>
      <UiIconButton variant="ghost" size="sm" class="term-tab__add" @click="addTab">
        <Plus :size="14" />
      </UiIconButton>
    </div>

    <div v-if="searchVisible" class="term-search">
      <NInput
        ref="searchInputEl"
        v-model:value="searchQuery"
        placeholder="搜索…"
        size="small"
        style="width: 220px"
        @keydown.enter="searchNext"
        @keydown.escape="closeSearch"
      />
      <NCheckbox v-model:checked="searchCaseSensitive">区分大小写</NCheckbox>
      <NCheckbox v-model:checked="searchRegex">正则</NCheckbox>
      <UiButton variant="secondary" size="sm" @click="searchPrev">↑</UiButton>
      <UiButton variant="secondary" size="sm" @click="searchNext">↓</UiButton>
      <UiIconButton variant="ghost" size="sm" @click="closeSearch"><X :size="14" /></UiIconButton>
    </div>

    <div class="term-body">
      <div
        v-for="tab in tabs"
        :key="tab.id"
        :ref="el => { if (el) terminalEls[tab.id] = el as HTMLDivElement }"
        class="term-pane"
        :style="{ display: activeTabId === tab.id ? 'block' : 'none' }"
      />
      <div v-if="tabs.length === 0" class="term-empty">
        <EmptyBlock description="连接中…" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onMounted, onBeforeUnmount } from 'vue'
import { useRoute } from 'vue-router'
import { NInput, NCheckbox } from 'naive-ui'
import { Plus, X } from 'lucide-vue-next'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { SearchAddon } from '@xterm/addon-search'
import '@xterm/xterm/css/xterm.css'
import { useAuthStore } from '@/stores/auth'
import { useServerStore } from '@/stores/server'
import UiButton from '@/components/ui/UiButton.vue'
import UiIconButton from '@/components/ui/UiIconButton.vue'
import EmptyBlock from '@/components/ui/EmptyBlock.vue'

const route = useRoute()
const authStore = useAuthStore()
const serverStore = useServerStore()
const serverId = computed(() => Number(route.params.serverId))

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
  if (!serverStore.servers.length) await serverStore.fetch()
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
    cursorBlink: true, fontSize: 13,
    fontFamily: 'ui-monospace, "JetBrains Mono", Menlo, Monaco, monospace',
    theme: { background: '#0A0A0A', foreground: '#E4E4E7', cursor: '#3ECF8E' },
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
.term-page {
  display: flex; flex-direction: column;
  height: 100%;
  background: #0A0A0A;
  border: 1px solid var(--ui-border);
  border-radius: var(--radius-md);
  overflow: hidden;
}

.term-tabs {
  display: flex; align-items: center;
  gap: var(--space-1);
  padding: var(--space-2) var(--space-3);
  background: #111113;
  border-bottom: 1px solid #27272A;
  flex-shrink: 0;
  overflow-x: auto;
}

.term-tab {
  display: flex; align-items: center;
  gap: var(--space-2);
  padding: var(--space-1) var(--space-3);
  border-radius: var(--radius-sm);
  cursor: pointer;
  color: #71717A;
  font-size: var(--fs-xs);
  white-space: nowrap;
  transition: background var(--dur-fast) var(--ease), color var(--dur-fast) var(--ease);
  height: 28px;
}
.term-tab:hover { background: #18181B; color: #A1A1AA; }
.term-tab.is-active { background: #18181B; color: #E4E4E7; }

.term-tab__dot {
  width: 6px; height: 6px;
  border-radius: 50%;
  flex-shrink: 0;
}
.term-tab__dot.is-connecting { background: var(--ui-warning); }
.term-tab__dot.is-connected  { background: var(--ui-success); }
.term-tab__dot.is-disconnected { background: var(--ui-danger); }

.term-tab__close {
  opacity: 0.5;
  transition: opacity var(--dur-fast) var(--ease);
}
.term-tab__close:hover { opacity: 1; }

.term-tab__add { margin-left: var(--space-1); flex-shrink: 0; }

.term-search {
  display: flex; align-items: center;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-3);
  background: #111113;
  border-bottom: 1px solid #27272A;
  flex-shrink: 0;
}

.term-body {
  flex: 1;
  overflow: hidden;
  position: relative;
}
.term-pane {
  width: 100%; height: 100%;
  padding: var(--space-2);
}
.term-empty {
  display: flex; align-items: center; justify-content: center;
  height: 100%;
}

:deep(.xterm) { height: 100%; }
:deep(.xterm-viewport) { background-color: #0A0A0A !important; }
</style>
