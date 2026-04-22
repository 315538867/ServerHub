<template>
  <div class="db-layout">
    <!-- 左侧边栏 -->
    <UiCard padding="none" class="db-sidebar">
      <div class="db-side-head">
        <span>数据库连接</span>
        <UiIconButton variant="ghost" size="sm" aria-label="新增连接" title="新增连接" @click="openAddConn">
          <Plus :size="14" />
        </UiIconButton>
      </div>
      <div class="sidebar-list">
        <div
          v-for="c in conns"
          :key="c.id"
          class="conn-item"
          :class="{ active: selectedConn?.id === c.id }"
          @click="selectConn(c)"
        >
          <div class="conn-item__icon" :class="c.type === 'redis' ? 'conn-item__icon--redis' : 'conn-item__icon--mysql'">
            <Database v-if="c.type !== 'redis'" :size="14" />
            <Server v-else :size="14" />
          </div>
          <div class="conn-info">
            <div class="conn-name">{{ c.name }}</div>
            <div class="conn-meta">{{ c.type.toUpperCase() }} · {{ c.host }}:{{ c.port }}</div>
          </div>
          <NPopconfirm @positive-click="deleteConnItem(c)" positive-text="删除" negative-text="取消">
            <template #trigger>
              <UiIconButton variant="ghost" size="sm" class="conn-delete-btn" @click.stop>
                <Trash2 :size="13" />
              </UiIconButton>
            </template>
            确认删除连接 {{ c.name }} ？
          </NPopconfirm>
        </div>
        <EmptyBlock v-if="conns.length === 0" description="暂无连接" />
      </div>
    </UiCard>

    <!-- 右侧主面板 -->
    <div class="db-main">
      <template v-if="selectedConn">
        <UiCard padding="md">
          <div class="db-main-head">
            <div class="db-main-title">
              <span>{{ selectedConn.name }}</span>
              <UiBadge :tone="selectedConn.type === 'redis' ? 'warning' : 'brand'">
                {{ selectedConn.type.toUpperCase() }}
              </UiBadge>
            </div>
            <UiButton variant="secondary" size="sm" @click="testConnection">测试连接</UiButton>
          </div>
        </UiCard>

        <!-- MySQL 视图 -->
        <UiCard v-if="selectedConn.type === 'mysql'" padding="md">
          <NTabs v-model:value="mysqlTab" type="line" size="small" animated>
            <NTabPane name="databases" tab="数据库">
              <div class="tab-toolbar">
                <UiButton variant="primary" size="sm" @click="openCreateDb">
                  <template #icon><Plus :size="14" /></template>
                  建库
                </UiButton>
                <UiButton variant="secondary" size="sm" :loading="dbLoading" @click="loadDatabases">
                  <template #icon><RefreshCw :size="14" /></template>
                  刷新
                </UiButton>
              </div>
              <NDataTable :columns="dbTableColumns" :data="databaseRows" :loading="dbLoading" :row-key="(r: any) => r.name" size="small" :bordered="false" />
            </NTabPane>

            <NTabPane name="users" tab="用户">
              <div class="tab-toolbar">
                <UiButton variant="primary" size="sm" @click="openCreateUser">
                  <template #icon><Plus :size="14" /></template>
                  添加用户
                </UiButton>
                <UiButton variant="secondary" size="sm" :loading="userLoading" @click="loadUsers">
                  <template #icon><RefreshCw :size="14" /></template>
                  刷新
                </UiButton>
              </div>
              <NDataTable :columns="userColumns" :data="users" :loading="userLoading" :row-key="(r: any) => r.user + r.host" size="small" :bordered="false" />
            </NTabPane>

            <NTabPane name="query" tab="SQL 执行器">
              <div class="query-toolbar">
                <NSelect v-model:value="queryDb" placeholder="选择数据库" size="small" style="width:200px" clearable :options="dbOptions" />
                <UiButton variant="primary" size="sm" :loading="queryLoading" @click="runQuery">执行</UiButton>
              </div>
              <div ref="sqlEditorEl" class="sql-editor" />
              <div v-if="queryResult" class="query-result">
                <NDataTable :columns="queryColumnsData" :data="queryRowsData" size="small" :bordered="false" max-height="300" :row-key="(r: any) => r._idx" />
                <div class="result-meta">共 {{ queryResult.rows.length }} 行</div>
              </div>
              <div v-if="queryError" class="query-error">{{ queryError }}</div>
            </NTabPane>

            <NTabPane name="status" tab="状态">
              <div class="tab-toolbar">
                <UiButton variant="secondary" size="sm" :loading="statusLoading" @click="loadStatus">
                  <template #icon><RefreshCw :size="14" /></template>
                  刷新
                </UiButton>
              </div>
              <NDataTable :columns="statusColumns" :data="statusRowsData" :loading="statusLoading" :row-key="(r: any) => r.key" size="small" :bordered="false" max-height="500" />
            </NTabPane>
          </NTabs>
        </UiCard>

        <!-- Redis 视图 -->
        <UiCard v-else-if="selectedConn.type === 'redis'" padding="md">
          <NTabs v-model:value="redisTab" type="line" size="small" animated>
            <NTabPane name="info" tab="状态">
              <div class="tab-toolbar">
                <UiButton variant="secondary" size="sm" :loading="infoLoading" @click="loadRedisInfo">
                  <template #icon><RefreshCw :size="14" /></template>
                  刷新
                </UiButton>
              </div>
              <div class="redis-info-grid">
                <div v-for="(val, key) in redisInfo" :key="key" class="info-item">
                  <span class="info-key">{{ key }}</span>
                  <span class="info-val">{{ val }}</span>
                </div>
              </div>
            </NTabPane>

            <NTabPane name="keys" tab="Key 浏览">
              <div class="tab-toolbar">
                <NInput v-model:value="keyPattern" placeholder="搜索 Pattern（默认 *）" size="small" style="width:220px" />
                <UiButton variant="secondary" size="sm" :loading="keysLoading" @click="loadKeys">搜索</UiButton>
                <NPopconfirm @positive-click="doFlushDB" positive-text="执行" negative-text="取消">
                  <template #trigger>
                    <UiButton variant="danger" size="sm">FLUSHDB</UiButton>
                  </template>
                  确认 FLUSHDB？所有数据将被清空！
                </NPopconfirm>
              </div>
              <div class="keys-layout">
                <div class="keys-list">
                  <div
                    v-for="k in redisKeys"
                    :key="k"
                    class="key-item"
                    :class="{ active: selectedKey === k }"
                    @click="viewKey(k)"
                  >{{ k }}</div>
                </div>
                <div class="key-detail" v-if="keyDetail">
                  <div class="key-detail-header">
                    <UiBadge tone="neutral">{{ keyDetail.type }}</UiBadge>
                    <span class="key-ttl">TTL: {{ keyDetail.ttl }}s</span>
                    <UiButton variant="danger" size="sm" @click="deleteKey(selectedKey!)">删除</UiButton>
                  </div>
                  <pre class="key-value">{{ keyDetail.value }}</pre>
                </div>
              </div>
            </NTabPane>
          </NTabs>
        </UiCard>
      </template>

      <UiCard v-else padding="lg" class="db-empty">
        <EmptyBlock description="选择或创建一个数据库连接" />
      </UiCard>
    </div>

    <!-- 添加连接弹窗 -->
    <NModal v-model:show="addConnVisible" preset="card" title="添加数据库连接" style="width: 480px" :bordered="false">
      <NForm :model="connForm" label-placement="left" label-width="80">
        <NFormItem label="名称">
          <NInput v-model:value="connForm.name" placeholder="My MySQL" />
        </NFormItem>
        <NFormItem label="服务器">
          <NSelect v-model:value="connForm.server_id" placeholder="选择服务器" :options="serverOptions" />
        </NFormItem>
        <NFormItem label="类型">
          <NRadioGroup v-model:value="connForm.type">
            <NRadio value="mysql">MySQL</NRadio>
            <NRadio value="redis">Redis</NRadio>
          </NRadioGroup>
        </NFormItem>
        <NFormItem label="Host">
          <NInput v-model:value="connForm.host" placeholder="127.0.0.1 / localhost" />
          <template #feedback>
            <span style="font-size: var(--fs-xs); color: var(--ui-fg-3)">
              填 <code>localhost</code> 走 Unix socket（适合只允许 <code>'user'@'localhost'</code> 的库）；填 <code>127.0.0.1</code> 走 TCP。
            </span>
          </template>
        </NFormItem>
        <NFormItem label="端口">
          <NInputNumber v-model:value="connForm.port" :min="1" :max="65535" :disabled="connForm.host?.toLowerCase() === 'localhost'" style="width:100%" />
        </NFormItem>
        <NFormItem v-if="connForm.type === 'mysql'" label="用户名">
          <NInput v-model:value="connForm.username" placeholder="root" />
        </NFormItem>
        <NFormItem label="密码">
          <NInput v-model:value="connForm.password" type="password" show-password-on="click" />
        </NFormItem>
        <NFormItem v-if="connForm.type === 'mysql'" label="默认库">
          <NInput v-model:value="connForm.database" placeholder="可选" />
        </NFormItem>
      </NForm>
      <template #footer>
        <div class="modal-foot">
          <UiButton variant="secondary" size="sm" @click="addConnVisible = false">取消</UiButton>
          <UiButton variant="primary" size="sm" :loading="addConnLoading" @click="confirmAddConn">保存</UiButton>
        </div>
      </template>
    </NModal>

    <!-- 创建数据库弹窗 -->
    <NModal v-model:show="createDbVisible" preset="card" title="创建数据库" style="width: 400px" :bordered="false">
      <NForm :model="createDbForm" label-placement="left" label-width="80">
        <NFormItem label="库名">
          <NInput v-model:value="createDbForm.name" placeholder="mydb" />
        </NFormItem>
        <NFormItem label="字符集">
          <NSelect v-model:value="createDbForm.charset" :options="charsetOptions" />
        </NFormItem>
      </NForm>
      <template #footer>
        <div class="modal-foot">
          <UiButton variant="secondary" size="sm" @click="createDbVisible = false">取消</UiButton>
          <UiButton variant="primary" size="sm" :loading="createDbLoading" @click="confirmCreateDb">创建</UiButton>
        </div>
      </template>
    </NModal>

    <!-- 添加用户弹窗 -->
    <NModal v-model:show="createUserVisible" preset="card" title="添加用户" style="width: 400px" :bordered="false">
      <NForm :model="createUserForm" label-placement="left" label-width="80">
        <NFormItem label="用户名">
          <NInput v-model:value="createUserForm.user" placeholder="appuser" />
        </NFormItem>
        <NFormItem label="Host">
          <NInput v-model:value="createUserForm.host" placeholder="%" />
        </NFormItem>
        <NFormItem label="密码">
          <NInput v-model:value="createUserForm.password" type="password" show-password-on="click" />
        </NFormItem>
        <NFormItem label="授权库">
          <NInput v-model:value="createUserForm.database" placeholder="留空为 *.*" />
        </NFormItem>
        <NFormItem label="权限">
          <NSelect v-model:value="createUserForm.grant" :options="grantOptions" />
        </NFormItem>
      </NForm>
      <template #footer>
        <div class="modal-foot">
          <UiButton variant="secondary" size="sm" @click="createUserVisible = false">取消</UiButton>
          <UiButton variant="primary" size="sm" :loading="createUserLoading" @click="confirmCreateUser">创建</UiButton>
        </div>
      </template>
    </NModal>
  </div>
</template>

<script setup lang="ts">
import { h, ref, computed, watch, nextTick, onBeforeUnmount } from 'vue'
import {
  NTabs, NTabPane, NDataTable, NModal, NForm, NFormItem, NInput, NInputNumber,
  NSelect, NRadioGroup, NRadio, NPopconfirm, useMessage,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { Plus, RefreshCw, Trash2, Database, Server } from 'lucide-vue-next'
import { EditorView, basicSetup } from 'codemirror'
import { EditorState } from '@codemirror/state'
import { sql } from '@codemirror/lang-sql'
import { oneDark } from '@codemirror/theme-one-dark'
import { useAuthStore } from '@/stores/auth'
import { getServers } from '@/api/servers'
import {
  listConns, createConn, deleteConn, testConn,
  mysqlDatabases, mysqlCreateDatabase, mysqlDropDatabase,
  mysqlUsers, mysqlCreateUser, mysqlQuery, mysqlStatus, mysqlExportUrl,
  redisInfo as apiRedisInfo, redisKeys as apiRedisKeys, redisGetKey, redisDelKey, redisFlushDB,
} from '@/api/database'
import type { DBConn } from '@/api/database'
import type { Server as ServerType } from '@/types/api'
import UiCard from '@/components/ui/UiCard.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiIconButton from '@/components/ui/UiIconButton.vue'
import EmptyBlock from '@/components/ui/EmptyBlock.vue'

const auth = useAuthStore()
const message = useMessage()
const servers = ref<ServerType[]>([])
const conns = ref<DBConn[]>([])
const selectedConn = ref<DBConn | null>(null)

const serverOptions = computed(() =>
  servers.value.map(s => ({ label: `${s.name} (${s.host})`, value: s.id })))

const charsetOptions = [
  { label: 'utf8mb4', value: 'utf8mb4' },
  { label: 'utf8', value: 'utf8' },
  { label: 'latin1', value: 'latin1' },
]
const grantOptions = [
  { label: 'ALL PRIVILEGES', value: 'ALL PRIVILEGES' },
  { label: 'SELECT', value: 'SELECT' },
  { label: 'SELECT, INSERT, UPDATE, DELETE', value: 'SELECT, INSERT, UPDATE, DELETE' },
]

// ── Connections ──────────────────────────────────────────────────
const addConnVisible = ref(false)
const addConnLoading = ref(false)
const connForm = ref({ name: '', server_id: 0, type: 'mysql' as 'mysql' | 'redis', host: '127.0.0.1', port: 3306, username: 'root', password: '', database: '' })

watch(() => connForm.value.type, (t) => {
  connForm.value.port = t === 'redis' ? 6379 : 3306
})

function openAddConn() {
  connForm.value = { name: '', server_id: servers.value[0]?.id ?? 0, type: 'mysql', host: '127.0.0.1', port: 3306, username: 'root', password: '', database: '' }
  addConnVisible.value = true
}

async function confirmAddConn() {
  if (!connForm.value.name || !connForm.value.server_id) {
    message.warning('请填写名称并选择服务器')
    return
  }
  addConnLoading.value = true
  try {
    await createConn(connForm.value)
    message.success('连接已创建')
    addConnVisible.value = false
    await loadConns()
  } catch (e: any) {
    message.error(e?.response?.data?.msg ?? '创建失败')
  } finally { addConnLoading.value = false }
}

async function deleteConnItem(c: DBConn) {
  await deleteConn(c.id)
  if (selectedConn.value?.id === c.id) selectedConn.value = null
  await loadConns()
}

function selectConn(c: DBConn) {
  destroySqlEditor()
  selectedConn.value = c
  mysqlTab.value = 'databases'
  redisTab.value = 'info'
  if (c.type === 'mysql') loadDatabases()
  else loadRedisInfo()
}

function destroySqlEditor() {
  sqlEditor?.destroy()
  sqlEditor = null
}

async function testConnection() {
  if (!selectedConn.value) return
  try {
    const res = await testConn(selectedConn.value.id)
    message.success(`连接成功: ${res?.output ?? 'OK'}`)
  } catch (e: any) {
    message.error(e?.response?.data?.msg ?? '连接失败')
  }
}

async function loadConns() { conns.value = await listConns() }

// ── MySQL ────────────────────────────────────────────────────────
const mysqlTab = ref('databases')
const dbLoading = ref(false)
const databases = ref<string[]>([])
const databaseRows = computed(() => databases.value.map(name => ({ name })))
const dbOptions = computed(() => databases.value.map(d => ({ label: d, value: d })))
const userLoading = ref(false)
const users = ref<Array<{ user: string; host: string }>>([])
const statusLoading = ref(false)
const statusRowsData = ref<Array<{ key: string; val: string }>>([])
const queryLoading = ref(false)
const queryDb = ref('')
const queryResult = ref<{ columns: string[]; rows: string[][] } | null>(null)
const queryError = ref('')
const sqlEditorEl = ref<HTMLDivElement>()
let sqlEditor: EditorView | null = null

const dbTableColumns = computed<DataTableColumns<{ name: string }>>(() => [
  { title: '数据库名', key: 'name', minWidth: 200 },
  {
    title: '操作', key: 'ops', width: 160, fixed: 'right' as const, align: 'center' as const,
    render: (row) => h('div', { class: 'cell-ops' }, [
      h(UiButton, { variant: 'ghost', size: 'sm', onClick: () => exportDatabase(row.name) }, () => '导出'),
      h(NPopconfirm, {
        onPositiveClick: () => dropDatabase(row.name),
        positiveText: '删除', negativeText: '取消',
      }, {
        trigger: () => h(UiButton, { variant: 'ghost', size: 'sm' },
          () => h('span', { class: 'text-danger' }, '删除')),
        default: () => `确认删除数据库 ${row.name}？不可恢复！`,
      }),
    ]),
  },
])
const userColumns: DataTableColumns<{ user: string; host: string }> = [
  { title: '用户名', key: 'user', minWidth: 150 },
  { title: 'Host', key: 'host', width: 160 },
]
const statusColumns: DataTableColumns<{ key: string; val: string }> = [
  { title: '变量名', key: 'key', width: 300, ellipsis: { tooltip: true } },
  { title: '值', key: 'val', minWidth: 200, ellipsis: { tooltip: true } },
]

const queryRowsData = computed(() => {
  if (!queryResult.value) return []
  return queryResult.value.rows.map((row, idx) => {
    const obj: Record<string, string> = { _idx: String(idx) }
    queryResult.value!.columns.forEach((col, i) => { obj[col] = row[i] })
    return obj
  })
})
const queryColumnsData = computed<DataTableColumns<any>>(() =>
  queryResult.value?.columns.map(col => ({ title: col, key: col, minWidth: 120, ellipsis: { tooltip: true } })) ?? []
)

watch(mysqlTab, async (tab) => {
  if (!selectedConn.value) return
  if (tab === 'databases') loadDatabases()
  if (tab === 'users') loadUsers()
  if (tab === 'status') loadStatus()
  if (tab === 'query') { await nextTick(); initSqlEditor() }
})

function initSqlEditor() {
  if (!sqlEditorEl.value) return
  if (sqlEditor && !sqlEditorEl.value.contains(sqlEditor.dom)) {
    sqlEditor.destroy()
    sqlEditor = null
  }
  if (sqlEditor) return
  sqlEditor = new EditorView({
    state: EditorState.create({ doc: 'SELECT 1', extensions: [basicSetup, sql(), oneDark] }),
    parent: sqlEditorEl.value,
  })
}

async function loadDatabases() {
  if (!selectedConn.value) return
  dbLoading.value = true
  try { databases.value = await mysqlDatabases(selectedConn.value.id) }
  finally { dbLoading.value = false }
}

async function loadUsers() {
  if (!selectedConn.value) return
  userLoading.value = true
  try { users.value = await mysqlUsers(selectedConn.value.id) }
  finally { userLoading.value = false }
}

async function loadStatus() {
  if (!selectedConn.value) return
  statusLoading.value = true
  try {
    const res = await mysqlStatus(selectedConn.value.id)
    statusRowsData.value = res.rows.map((row: string[]) => ({ key: row[0], val: row[1] }))
  } finally { statusLoading.value = false }
}

async function runQuery() {
  if (!selectedConn.value || !sqlEditor) return
  queryLoading.value = true
  queryResult.value = null
  queryError.value = ''
  try {
    const sqlText = sqlEditor.state.doc.toString()
    queryResult.value = await mysqlQuery(selectedConn.value.id, sqlText, queryDb.value || undefined)
  } catch (e: any) {
    queryError.value = e?.response?.data?.msg ?? '查询失败'
  } finally { queryLoading.value = false }
}

const createDbVisible = ref(false)
const createDbLoading = ref(false)
const createDbForm = ref({ name: '', charset: 'utf8mb4' })

function openCreateDb() { createDbForm.value = { name: '', charset: 'utf8mb4' }; createDbVisible.value = true }

async function confirmCreateDb() {
  if (!selectedConn.value || !createDbForm.value.name) return
  createDbLoading.value = true
  try {
    await mysqlCreateDatabase(selectedConn.value.id, createDbForm.value.name, createDbForm.value.charset)
    message.success('数据库已创建')
    createDbVisible.value = false
    await loadDatabases()
  } catch (e: any) { message.error(e?.response?.data?.msg ?? '创建失败') }
  finally { createDbLoading.value = false }
}

async function dropDatabase(dbname: string) {
  if (!selectedConn.value) return
  try {
    await mysqlDropDatabase(selectedConn.value.id, dbname)
    message.success('已删除')
    await loadDatabases()
  } catch (e: any) { message.error(e?.response?.data?.msg ?? '删除失败') }
}

function exportDatabase(dbname: string) {
  if (!selectedConn.value) return
  const url = mysqlExportUrl(selectedConn.value.id, dbname, auth.token)
  window.open(url, '_blank')
}

const createUserVisible = ref(false)
const createUserLoading = ref(false)
const createUserForm = ref({ user: '', host: '%', password: '', database: '', grant: 'ALL PRIVILEGES' })

function openCreateUser() { createUserForm.value = { user: '', host: '%', password: '', database: '', grant: 'ALL PRIVILEGES' }; createUserVisible.value = true }

async function confirmCreateUser() {
  if (!selectedConn.value || !createUserForm.value.user || !createUserForm.value.password) {
    message.warning('请填写用户名和密码')
    return
  }
  createUserLoading.value = true
  try {
    await mysqlCreateUser(selectedConn.value.id, createUserForm.value)
    message.success('用户已创建')
    createUserVisible.value = false
    await loadUsers()
  } catch (e: any) { message.error(e?.response?.data?.msg ?? '创建失败') }
  finally { createUserLoading.value = false }
}

// ── Redis ─────────────────────────────────────────────────────────
const redisTab = ref('info')
const infoLoading = ref(false)
const redisInfo = ref<Record<string, string>>({})
const keysLoading = ref(false)
const redisKeys = ref<string[]>([])
const keyPattern = ref('*')
const selectedKey = ref<string | null>(null)
const keyDetail = ref<{ type: string; value: string; ttl: string } | null>(null)

watch(redisTab, (tab) => {
  if (!selectedConn.value) return
  if (tab === 'info') loadRedisInfo()
  if (tab === 'keys') loadKeys()
})

async function loadRedisInfo() {
  if (!selectedConn.value) return
  infoLoading.value = true
  try { redisInfo.value = await apiRedisInfo(selectedConn.value.id) }
  finally { infoLoading.value = false }
}

async function loadKeys() {
  if (!selectedConn.value) return
  keysLoading.value = true
  try { redisKeys.value = await apiRedisKeys(selectedConn.value.id, keyPattern.value || '*') }
  finally { keysLoading.value = false }
}

async function viewKey(key: string) {
  if (!selectedConn.value) return
  selectedKey.value = key
  keyDetail.value = await redisGetKey(selectedConn.value.id, key)
}

async function deleteKey(key: string) {
  if (!selectedConn.value || !key) return
  await redisDelKey(selectedConn.value.id, key)
  keyDetail.value = null
  selectedKey.value = null
  await loadKeys()
}

async function doFlushDB() {
  if (!selectedConn.value) return
  try {
    await redisFlushDB(selectedConn.value.id)
    message.success('FLUSHDB 已执行')
    await loadKeys()
  } catch (e: any) { message.error(e?.response?.data?.msg ?? '失败') }
}

// ── Init ─────────────────────────────────────────────────────────
onBeforeUnmount(() => { sqlEditor?.destroy(); sqlEditor = null })

async function init() {
  servers.value = await getServers()
  await loadConns()
}
init()
</script>

<style scoped>
.db-layout {
  display: flex;
  gap: var(--space-4);
  padding: var(--space-6);
  min-height: calc(100vh - 60px);
  align-items: flex-start;
}

.db-sidebar {
  width: 240px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.db-side-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--space-3) var(--space-4);
  font-size: var(--fs-sm);
  font-weight: var(--fw-semibold);
  color: var(--ui-fg);
  border-bottom: 1px solid var(--ui-border);
}

.sidebar-list {
  flex: 1;
  overflow-y: auto;
  padding: var(--space-2) 0;
}

.conn-item {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-3);
  cursor: pointer;
  transition: background var(--dur-fast) var(--ease);
  position: relative;
}
.conn-item:hover { background: var(--ui-bg-2); }
.conn-item.active {
  background: var(--ui-brand-soft, rgba(62,207,142,.08));
}
.conn-item.active .conn-name { color: var(--ui-brand-fg); }

.conn-item__icon {
  width: 28px; height: 28px;
  border-radius: var(--radius-sm);
  display: grid; place-items: center;
  flex-shrink: 0;
}
.conn-item__icon--mysql {
  background: rgba(62,207,142,.12);
  color: var(--ui-brand-fg);
}
.conn-item__icon--redis {
  background: rgba(245,158,11,.12);
  color: var(--ui-warning-fg);
}

.conn-info { flex: 1; min-width: 0; }
.conn-name {
  font-size: var(--fs-sm);
  font-weight: var(--fw-medium);
  color: var(--ui-fg);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.conn-meta {
  font-size: var(--fs-xs);
  color: var(--ui-fg-3);
  margin-top: 2px;
}
.conn-delete-btn { opacity: 0; transition: opacity var(--dur-fast) var(--ease); }
.conn-item:hover .conn-delete-btn { opacity: 1; }

.db-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}

.db-main-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.db-main-title {
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
  font-size: var(--fs-md);
  font-weight: var(--fw-semibold);
  color: var(--ui-fg);
}

.db-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 360px;
}

.tab-toolbar {
  display: flex;
  gap: var(--space-2);
  margin-bottom: var(--space-3);
  align-items: center;
}
.query-toolbar {
  display: flex;
  gap: var(--space-2);
  margin-bottom: var(--space-2);
  align-items: center;
}

.sql-editor {
  height: 220px;
  overflow: auto;
  border: 1px solid var(--ui-border);
  border-radius: var(--radius-sm);
  margin-bottom: var(--space-3);
}
:deep(.cm-editor) { height: 100%; }
:deep(.cm-scroller) { overflow: auto; }

.query-result { margin-top: var(--space-2); }
.result-meta {
  font-size: var(--fs-xs);
  color: var(--ui-fg-3);
  margin-top: var(--space-2);
}
.query-error {
  color: var(--ui-danger-fg);
  background: rgba(239,68,68,.08);
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-sm);
  font-size: var(--fs-sm);
  margin-top: var(--space-2);
}

.redis-info-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
  gap: var(--space-2);
}
.info-item {
  display: flex;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-3);
  background: var(--ui-bg-2);
  border-radius: var(--radius-sm);
  font-size: var(--fs-xs);
}
.info-key { color: var(--ui-fg-3); min-width: 160px; flex-shrink: 0; }
.info-val {
  color: var(--ui-fg);
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  font-family: var(--font-mono);
}

.keys-layout {
  display: flex;
  gap: var(--space-3);
  height: 420px;
}
.keys-list {
  width: 260px;
  flex-shrink: 0;
  overflow-y: auto;
  border: 1px solid var(--ui-border);
  border-radius: var(--radius-sm);
}
.key-item {
  padding: var(--space-2) var(--space-3);
  font-size: var(--fs-xs);
  font-family: var(--font-mono);
  cursor: pointer;
  border-bottom: 1px solid var(--ui-border);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--ui-fg-2);
}
.key-item:hover { background: var(--ui-bg-2); }
.key-item.active {
  background: rgba(62,207,142,.08);
  color: var(--ui-brand-fg);
}

.key-detail {
  flex: 1;
  border: 1px solid var(--ui-border);
  border-radius: var(--radius-sm);
  padding: var(--space-3);
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  overflow: auto;
}
.key-detail-header {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}
.key-ttl {
  font-size: var(--fs-xs);
  color: var(--ui-fg-3);
  flex: 1;
}
.key-value {
  flex: 1;
  background: #0A0A0A;
  color: #86efac;
  padding: var(--space-3);
  border-radius: var(--radius-sm);
  font-size: var(--fs-xs);
  font-family: var(--font-mono);
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-all;
  margin: 0;
}

.modal-foot {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-2);
}

:deep(.cell-ops) { display: inline-flex; gap: var(--space-1); }
:deep(.text-danger) { color: var(--ui-danger-fg); }
</style>
