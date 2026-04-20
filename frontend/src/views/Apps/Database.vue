<template>
  <div class="page-container">
    <template v-if="conn">
      <!-- 连接信息 -->
      <div class="section-block">
        <div class="section-title">
          <span class="title-text">
            数据库连接
            <t-tag :theme="testTheme" variant="light" size="small" class="conn-status">
              <span class="dot" :class="testResult || 'unknown'" />
              {{ testResult === 'ok' ? '在线' : testResult === 'fail' ? '离线' : '未检测' }}
            </t-tag>
          </span>
          <t-space size="small">
            <t-button size="small" :loading="testing" @click="doTest">连接测试</t-button>
            <t-button size="small" variant="outline" @click="goGlobalDB">管理全部连接</t-button>
            <t-button v-if="canUnbind" size="small" variant="outline" theme="danger" @click="unbindConn">解除绑定</t-button>
          </t-space>
        </div>
        <div class="desc-wrap">
          <t-descriptions :column="2">
            <t-descriptions-item label="名称">{{ conn.name }}</t-descriptions-item>
            <t-descriptions-item label="类型"><t-tag size="small" variant="light">{{ conn.type.toUpperCase() }}</t-tag></t-descriptions-item>
            <t-descriptions-item label="主机"><code class="mono">{{ conn.host }}:{{ conn.port }}</code></t-descriptions-item>
            <t-descriptions-item label="用户">{{ conn.username || '—' }}</t-descriptions-item>
            <t-descriptions-item label="数据库">{{ conn.database || '—' }}</t-descriptions-item>
            <t-descriptions-item label="服务器">{{ serverName }}</t-descriptions-item>
          </t-descriptions>
        </div>
      </div>

      <!-- MySQL 数据库列表 -->
      <template v-if="conn.type === 'mysql'">
        <div class="section-block">
          <div class="section-title">
            <span class="title-text">数据库列表 <span class="count-badge">{{ databases.length }}</span></span>
            <t-space size="small">
              <t-input
                v-model="dbFilter"
                size="small" clearable
                placeholder="过滤库名"
                class="filter-input"
              >
                <template #prefix-icon><t-icon name="search" /></template>
              </t-input>
              <t-button size="small" theme="primary" @click="openCreateDB">新建</t-button>
              <t-button size="small" variant="outline" :loading="dbLoading" @click="loadDBs">刷新</t-button>
            </t-space>
          </div>
          <div class="table-wrap">
            <t-table :data="filteredDatabases" :columns="dbColumns" :loading="dbLoading" row-key="name" stripe size="small" empty="暂无数据库">
              <template #name="{ row }"><code class="mono">{{ row.name }}</code></template>
              <template #operations="{ row }">
                <t-popconfirm :content="`确认删除数据库 ${row.name}？此操作不可恢复`" @confirm="dropDB(row.name)">
                  <t-button theme="danger" size="small" variant="text">删除</t-button>
                </t-popconfirm>
              </template>
            </t-table>
          </div>
        </div>
      </template>

      <!-- Redis 状态 -->
      <template v-if="conn.type === 'redis'">
        <div class="section-block">
          <div class="section-title">
            <span class="title-text">Redis 状态</span>
            <t-space size="small">
              <t-input
                v-model="redisFilter"
                size="small" clearable
                placeholder="过滤键"
                class="filter-input"
              >
                <template #prefix-icon><t-icon name="search" /></template>
              </t-input>
              <t-button size="small" variant="outline" :loading="redisLoading" @click="loadRedisInfo">刷新</t-button>
            </t-space>
          </div>
          <div class="table-wrap">
            <t-table :data="filteredRedisRows" :columns="redisColumns" :loading="redisLoading" row-key="key" stripe size="small" empty="暂无数据">
              <template #key="{ row }"><code class="mono">{{ row.key }}</code></template>
            </t-table>
          </div>
        </div>
      </template>
    </template>

    <!-- ── 未绑定：新版友好空态 ─────────────────────────────────────── -->
    <div v-else-if="!loading" class="empty-hero">
      <div class="eh-icon">🗄️</div>
      <div class="eh-title">该应用未关联数据库连接</div>
      <div class="eh-subtitle">你可以绑定一个现有连接，或新建一个</div>

      <div class="eh-actions">
        <!-- 绑定现有 -->
        <div class="eh-card">
          <div class="eh-card-title">🔗 绑定现有连接</div>
          <div class="eh-card-body">
            <t-select
              v-model="bindConnId"
              placeholder="选择连接"
              :options="availableConnOptions"
              size="small"
              clearable
              :disabled="availableConnOptions.length === 0"
            />
            <div v-if="availableConnOptions.length === 0" class="eh-card-hint">
              {{ app?.server_id ? '该服务器下暂无数据库连接' : '应用尚未绑定服务器' }}
            </div>
            <t-button
              theme="primary"
              size="small"
              :disabled="!bindConnId"
              :loading="binding"
              @click="doBind"
            >绑定</t-button>
          </div>
        </div>

        <!-- 新建 -->
        <div class="eh-card">
          <div class="eh-card-title">➕ 新建连接</div>
          <div class="eh-card-body">
            <div class="eh-card-hint">跳转到「全局数据库」管理页，新建一个连接后回到本 Tab 绑定</div>
            <t-button size="small" variant="outline" @click="goGlobalDB">去新建</t-button>
          </div>
        </div>
      </div>
    </div>

    <!-- 新建数据库对话框 -->
    <t-dialog
      v-model:visible="createDBVisible"
      header="新建数据库"
      width="400px"
      confirm-btn="创建"
      @confirm="confirmCreateDB"
    >
      <t-form :data="createDBForm" label-width="80px" colon>
        <t-form-item label="库名"><t-input v-model="createDBForm.name" placeholder="my_database" /></t-form-item>
        <t-form-item label="字符集"><t-input v-model="createDBForm.charset" placeholder="utf8mb4（默认）" /></t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import { useAppStore } from '@/stores/app'
import { useServerStore } from '@/stores/server'
import { updateApp } from '@/api/application'
import { listConns, testConn, mysqlDatabases, mysqlCreateDatabase, mysqlDropDatabase, redisInfo } from '@/api/database'
import type { DBConn } from '@/api/database'

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const serverStore = useServerStore()
const appId = computed(() => Number(route.params.appId))
const app = computed(() => appStore.getById(appId.value))

const conn = ref<DBConn | null>(null)
const allConns = ref<DBConn[]>([])
const loading = ref(false)
const testing = ref(false)
const testResult = ref<'ok' | 'fail' | ''>('')

const databases = ref<{ name: string }[]>([])
const dbLoading = ref(false)
const dbFilter = ref('')

const redisInfoRows = ref<{ key: string; val: string }[]>([])
const redisLoading = ref(false)
const redisFilter = ref('')

const createDBVisible = ref(false)
const createDBForm = ref({ name: '', charset: '' })

// 绑定态
const bindConnId = ref<number | null>(null)
const binding = ref(false)

const dbColumns = [
  { colKey: 'name', title: '数据库名', minWidth: 200 },
  { colKey: 'operations', title: '操作', width: 120, fixed: 'right' as const },
]

const redisColumns = [
  { colKey: 'key', title: '键', width: 220 },
  { colKey: 'val', title: '值', minWidth: 300, ellipsis: true },
]

const testTheme = computed(() => {
  if (testResult.value === 'ok') return 'success'
  if (testResult.value === 'fail') return 'danger'
  return 'default'
})

const canUnbind = computed(() => !!app.value?.db_conn_id)

const serverName = computed(() => {
  const s = serverStore.getById(app.value?.server_id ?? -1)
  return s?.name || `#${app.value?.server_id ?? '—'}`
})

const availableConnOptions = computed(() => {
  const sid = app.value?.server_id
  return allConns.value
    .filter(c => !sid || c.server_id === sid)
    .map(c => ({
      label: `${c.name} · ${c.type.toUpperCase()} · ${c.host}:${c.port}`,
      value: c.id,
    }))
})

const filteredDatabases = computed(() => {
  const kw = dbFilter.value.trim().toLowerCase()
  return kw ? databases.value.filter(d => d.name.toLowerCase().includes(kw)) : databases.value
})

const filteredRedisRows = computed(() => {
  const kw = redisFilter.value.trim().toLowerCase()
  return kw ? redisInfoRows.value.filter(r => r.key.toLowerCase().includes(kw) || r.val.toLowerCase().includes(kw)) : redisInfoRows.value
})

async function loadAllConns() {
  try { allConns.value = (await listConns()) as unknown as DBConn[] }
  catch { allConns.value = [] }
}

async function loadConn() {
  if (!app.value?.db_conn_id) { conn.value = null; return }
  loading.value = true
  try {
    conn.value = allConns.value.find(c => c.id === app.value!.db_conn_id) ?? null
  } finally { loading.value = false }
}

async function doTest(silent = false) {
  if (!conn.value) return
  testing.value = true
  try { await testConn(conn.value.id); testResult.value = 'ok'; if (!silent) MessagePlugin.success('连接正常') }
  catch { testResult.value = 'fail'; if (!silent) MessagePlugin.error('连接失败') }
  finally { testing.value = false }
}

async function loadDBs() {
  if (!conn.value) return
  dbLoading.value = true
  try {
    const dbs = await mysqlDatabases(conn.value.id)
    databases.value = dbs.map(name => ({ name }))
  } catch { MessagePlugin.error('加载失败') }
  finally { dbLoading.value = false }
}

function openCreateDB() { createDBForm.value = { name: '', charset: '' }; createDBVisible.value = true }

async function confirmCreateDB() {
  if (!conn.value || !createDBForm.value.name) return
  try {
    await mysqlCreateDatabase(conn.value.id, createDBForm.value.name, createDBForm.value.charset || undefined)
    MessagePlugin.success('已创建'); createDBVisible.value = false; await loadDBs()
  } catch { MessagePlugin.error('创建失败') }
}

async function dropDB(name: string) {
  if (!conn.value) return
  try { await mysqlDropDatabase(conn.value.id, name); MessagePlugin.success('已删除'); await loadDBs() }
  catch { MessagePlugin.error('删除失败') }
}

async function loadRedisInfo() {
  if (!conn.value) return
  redisLoading.value = true
  try {
    const info = await redisInfo(conn.value.id)
    redisInfoRows.value = Object.entries(info).map(([key, val]) => ({ key, val }))
  } catch { MessagePlugin.error('加载失败') }
  finally { redisLoading.value = false }
}

async function doBind() {
  if (!bindConnId.value || !app.value) return
  binding.value = true
  try {
    await updateApp(app.value.id, { ...app.value, db_conn_id: bindConnId.value } as any)
    MessagePlugin.success('已绑定')
    await appStore.fetch()
    await loadConn()
    if (conn.value?.type === 'mysql') loadDBs()
    if (conn.value?.type === 'redis') loadRedisInfo()
    doTest(true)
  } catch { MessagePlugin.error('绑定失败') }
  finally { binding.value = false }
}

async function unbindConn() {
  if (!app.value) return
  try {
    await updateApp(app.value.id, { ...app.value, db_conn_id: null } as any)
    MessagePlugin.success('已解除绑定')
    await appStore.fetch()
    conn.value = null
    testResult.value = ''
  } catch { MessagePlugin.error('解除失败') }
}

function goGlobalDB() { router.push('/database') }

onMounted(async () => {
  if (!appStore.apps.length) await appStore.fetch()
  if (!serverStore.servers.length) serverStore.fetch()
  await loadAllConns()
  await loadConn()
  if (conn.value) {
    doTest(true)
    if (conn.value.type === 'mysql') loadDBs()
    if (conn.value.type === 'redis') loadRedisInfo()
  }
})
</script>

<style scoped>
.desc-wrap { padding: var(--sh-space-md) var(--sh-space-lg) var(--sh-space-lg); }
:deep(.t-descriptions__label) { color: var(--sh-text-secondary); font-size: 13px; width: 80px; }
:deep(.t-descriptions__content) { font-size: 13px; }
.table-wrap { padding: 0 var(--sh-space-lg) var(--sh-space-md); }
:deep(.t-table td) { font-size: 13px; }

.mono {
  font-family: var(--sh-font-mono, ui-monospace, SFMono-Regular, monospace);
  font-size: 12.5px;
  background: var(--sh-code-bg, rgba(0,0,0,.04));
  padding: 1px 6px;
  border-radius: 3px;
  color: var(--sh-text-primary);
}

.conn-status { margin-left: var(--sh-space-sm); display: inline-flex; align-items: center; gap: var(--sh-space-xs); }
.conn-status .dot {
  width: 6px; height: 6px; border-radius: 50%;
  background: #999;
  display: inline-block;
}
.conn-status .dot.ok { background: #67c23a; box-shadow: 0 0 0 2px color-mix(in srgb, #67c23a 25%, transparent); }
.conn-status .dot.fail { background: #e34d59; box-shadow: 0 0 0 2px color-mix(in srgb, #e34d59 25%, transparent); }

.count-badge {
  font-size: 11px;
  color: var(--sh-text-secondary);
  background: color-mix(in srgb, var(--sh-text-primary) 6%, transparent);
  padding: 1px 8px;
  border-radius: 10px;
  margin-left: var(--sh-space-sm);
  font-variant-numeric: tabular-nums;
}

.filter-input { width: 180px; }

/* 空态 */
.empty-hero {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: var(--sh-space-xl) var(--sh-space-lg);
  text-align: center;
  background: var(--sh-card-bg);
  border: 1px solid var(--sh-border);
  border-radius: 10px;
}
.eh-icon { font-size: 42px; margin-bottom: var(--sh-space-md); opacity: 0.85; }
.eh-title { font-size: 15px; font-weight: 600; color: var(--sh-text-primary); }
.eh-subtitle { font-size: 13px; color: var(--sh-text-secondary); margin-top: var(--sh-space-xs); }
.eh-actions {
  margin-top: var(--sh-space-lg);
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: var(--sh-space-md);
  width: 100%;
  max-width: 640px;
}
.eh-card {
  border: 1px solid var(--sh-border);
  border-radius: 8px;
  padding: var(--sh-space-md);
  background: color-mix(in srgb, var(--sh-text-primary) 2%, transparent);
  text-align: left;
  display: flex;
  flex-direction: column;
}
.eh-card-title { font-size: 13px; font-weight: 600; color: var(--sh-text-primary); margin-bottom: var(--sh-space-sm); }
.eh-card-body { display: flex; flex-direction: column; gap: var(--sh-space-sm); }
.eh-card-hint { font-size: 12px; color: var(--sh-text-secondary); line-height: 1.5; }
</style>
