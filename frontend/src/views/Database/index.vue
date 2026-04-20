<template>
  <div class="db-layout">
    <!-- 左侧边栏 -->
    <div class="db-sidebar section-block">
      <div class="section-title">
        <span>数据库连接</span>
        <t-button :icon="() => h(AddIcon)" shape="circle" size="small" variant="outline" @click="openAddConn" />
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
            <component-grid-icon v-if="c.type !== 'redis'" />
            <star-icon v-else />
          </div>
          <div class="conn-info">
            <div class="conn-name">{{ c.name }}</div>
            <div class="conn-meta">{{ c.type.toUpperCase() }} · {{ c.host }}:{{ c.port }}</div>
          </div>
          <t-tag size="small" :theme="c.type === 'redis' ? 'warning' : 'primary'" variant="light" class="conn-type-badge">
            {{ c.type.toUpperCase() }}
          </t-tag>
          <t-button shape="circle" size="small" theme="danger" variant="text" @click.stop="deleteConnItem(c)" class="conn-delete-btn">
            <template #icon><delete-icon /></template>
          </t-button>
        </div>
        <t-empty v-if="conns.length === 0" description="暂无连接" style="padding:24px 0" />
      </div>
    </div>

    <!-- 右侧主面板 -->
    <div class="db-main">
      <template v-if="selectedConn">
        <!-- 连接头部信息 -->
        <div class="section-block db-main-header">
          <div class="section-title">
            <div class="db-main-title">
              <span>{{ selectedConn.name }}</span>
              <t-tag :theme="selectedConn.type === 'redis' ? 'warning' : 'primary'" size="small" variant="light" style="margin-left:8px">
                {{ selectedConn.type.toUpperCase() }}
              </t-tag>
            </div>
            <t-button size="small" variant="outline" @click="testConnection">测试连接</t-button>
          </div>
        </div>

        <!-- MySQL 视图 -->
        <template v-if="selectedConn.type === 'mysql'">
          <div class="section-block">
            <t-tabs :value="mysqlTab" @change="val => (mysqlTab = val as string)">
              <t-tab-panel value="databases" label="数据库">
                <div class="tab-content">
                  <div class="tab-toolbar">
                    <t-button theme="primary" size="small" @click="openCreateDb">建库</t-button>
                    <t-button size="small" variant="outline" :loading="dbLoading" @click="loadDatabases">刷新</t-button>
                  </div>
                  <t-table :data="databaseRows" :columns="dbTableColumns" :loading="dbLoading" size="small" row-key="name">
                    <template #operations="{ row }">
                      <t-space size="small">
                        <t-button size="small" variant="text" @click="exportDatabase(row.name)">导出</t-button>
                        <t-popconfirm :content="`确认删除数据库 ${row.name}？不可恢复！`" @confirm="dropDatabase(row.name)">
                          <t-button theme="danger" size="small" variant="text">删除</t-button>
                        </t-popconfirm>
                      </t-space>
                    </template>
                  </t-table>
                </div>
              </t-tab-panel>

              <t-tab-panel value="users" label="用户">
                <div class="tab-content">
                  <div class="tab-toolbar">
                    <t-button theme="primary" size="small" @click="openCreateUser">添加用户</t-button>
                    <t-button size="small" variant="outline" :loading="userLoading" @click="loadUsers">刷新</t-button>
                  </div>
                  <t-table :data="users" :columns="userColumns" :loading="userLoading" size="small" row-key="user" />
                </div>
              </t-tab-panel>

              <t-tab-panel value="query" label="SQL 执行器">
                <div class="tab-content">
                  <div class="query-toolbar">
                    <t-select v-model="queryDb" placeholder="选择数据库" size="small" style="width:180px" clearable>
                      <t-option v-for="d in databases" :key="d" :label="d" :value="d" />
                    </t-select>
                    <t-button theme="primary" size="small" :loading="queryLoading" @click="runQuery">执行</t-button>
                  </div>
                  <div ref="sqlEditorEl" class="sql-editor" />
                  <div v-if="queryResult" class="query-result">
                    <t-table :data="queryRowsData" :columns="queryColumnsData" size="small" max-height="300" row-key="_idx" />
                    <div class="result-meta">共 {{ queryResult.rows.length }} 行</div>
                  </div>
                  <div v-if="queryError" class="query-error">{{ queryError }}</div>
                </div>
              </t-tab-panel>

              <t-tab-panel value="status" label="状态">
                <div class="tab-content">
                  <t-button size="small" variant="outline" :loading="statusLoading" @click="loadStatus" style="margin-bottom:12px">刷新</t-button>
                  <t-table :data="statusRowsData" :columns="statusColumns" :loading="statusLoading" size="small" max-height="500" row-key="key" />
                </div>
              </t-tab-panel>
            </t-tabs>
          </div>
        </template>

        <!-- Redis 视图 -->
        <template v-else-if="selectedConn.type === 'redis'">
          <div class="section-block">
            <t-tabs :value="redisTab" @change="val => (redisTab = val as string)">
              <t-tab-panel value="info" label="状态">
                <div class="tab-content">
                  <t-button size="small" variant="outline" :loading="infoLoading" @click="loadRedisInfo" style="margin-bottom:12px">刷新</t-button>
                  <div class="redis-info-grid">
                    <div v-for="(val, key) in redisInfo" :key="key" class="info-item">
                      <span class="info-key">{{ key }}</span>
                      <span class="info-val">{{ val }}</span>
                    </div>
                  </div>
                </div>
              </t-tab-panel>

              <t-tab-panel value="keys" label="Key 浏览">
                <div class="tab-content">
                  <div class="tab-toolbar">
                    <t-input v-model="keyPattern" placeholder="搜索 Pattern（默认 *）" size="small" style="width:220px" />
                    <t-button size="small" variant="outline" :loading="keysLoading" @click="loadKeys">搜索</t-button>
                    <t-popconfirm content="确认 FLUSHDB？所有数据将被清空！" @confirm="doFlushDB">
                      <t-button size="small" theme="danger" variant="outline">FLUSHDB</t-button>
                    </t-popconfirm>
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
                        <t-tag size="small" variant="light">{{ keyDetail.type }}</t-tag>
                        <span class="key-ttl">TTL: {{ keyDetail.ttl }}s</span>
                        <t-button size="small" theme="danger" variant="outline" @click="deleteKey(selectedKey!)">删除</t-button>
                      </div>
                      <pre class="key-value">{{ keyDetail.value }}</pre>
                    </div>
                  </div>
                </div>
              </t-tab-panel>
            </t-tabs>
          </div>
        </template>
      </template>

      <!-- 空状态 -->
      <div class="db-empty section-block" v-else>
        <t-empty description="选择或创建一个数据库连接" />
      </div>
    </div>

    <!-- 添加连接弹窗 -->
    <t-dialog
      v-model:visible="addConnVisible"
      header="添加数据库连接"
      width="480px"
      :confirm-btn="{ content: '保存', loading: addConnLoading }"
      @confirm="confirmAddConn"
    >
      <t-form :data="connForm" label-width="80px" colon>
        <t-form-item label="名称">
          <t-input v-model="connForm.name" placeholder="My MySQL" />
        </t-form-item>
        <t-form-item label="服务器">
          <t-select v-model="connForm.server_id" placeholder="选择服务器" style="width:100%">
            <t-option v-for="s in servers" :key="s.id" :label="`${s.name} (${s.host})`" :value="s.id" />
          </t-select>
        </t-form-item>
        <t-form-item label="类型">
          <t-radio-group v-model="connForm.type">
            <t-radio value="mysql">MySQL</t-radio>
            <t-radio value="redis">Redis</t-radio>
          </t-radio-group>
        </t-form-item>
        <t-form-item label="Host">
          <t-input v-model="connForm.host" placeholder="127.0.0.1" />
        </t-form-item>
        <t-form-item label="端口">
          <t-input-number v-model="connForm.port" :min="1" :max="65535" style="width:100%" />
        </t-form-item>
        <t-form-item v-if="connForm.type === 'mysql'" label="用户名">
          <t-input v-model="connForm.username" placeholder="root" />
        </t-form-item>
        <t-form-item label="密码">
          <t-input v-model="connForm.password" type="password" />
        </t-form-item>
        <t-form-item v-if="connForm.type === 'mysql'" label="默认库">
          <t-input v-model="connForm.database" placeholder="可选" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <!-- 创建数据库弹窗 -->
    <t-dialog
      v-model:visible="createDbVisible"
      header="创建数据库"
      width="400px"
      :confirm-btn="{ content: '创建', loading: createDbLoading }"
      @confirm="confirmCreateDb"
    >
      <t-form :data="createDbForm" label-width="80px" colon>
        <t-form-item label="库名">
          <t-input v-model="createDbForm.name" placeholder="mydb" />
        </t-form-item>
        <t-form-item label="字符集">
          <t-select v-model="createDbForm.charset" style="width:100%">
            <t-option label="utf8mb4" value="utf8mb4" />
            <t-option label="utf8" value="utf8" />
            <t-option label="latin1" value="latin1" />
          </t-select>
        </t-form-item>
      </t-form>
    </t-dialog>

    <!-- 添加用户弹窗 -->
    <t-dialog
      v-model:visible="createUserVisible"
      header="添加用户"
      width="400px"
      :confirm-btn="{ content: '创建', loading: createUserLoading }"
      @confirm="confirmCreateUser"
    >
      <t-form :data="createUserForm" label-width="80px" colon>
        <t-form-item label="用户名">
          <t-input v-model="createUserForm.user" placeholder="appuser" />
        </t-form-item>
        <t-form-item label="Host">
          <t-input v-model="createUserForm.host" placeholder="%" />
        </t-form-item>
        <t-form-item label="密码">
          <t-input v-model="createUserForm.password" type="password" />
        </t-form-item>
        <t-form-item label="授权库">
          <t-input v-model="createUserForm.database" placeholder="留空为 *.*" />
        </t-form-item>
        <t-form-item label="权限">
          <t-select v-model="createUserForm.grant" style="width:100%">
            <t-option label="ALL PRIVILEGES" value="ALL PRIVILEGES" />
            <t-option label="SELECT" value="SELECT" />
            <t-option label="SELECT, INSERT, UPDATE, DELETE" value="SELECT, INSERT, UPDATE, DELETE" />
          </t-select>
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { h, ref, computed, watch, nextTick, onBeforeUnmount } from 'vue'
import { AddIcon, DeleteIcon, ComponentGridIcon, StarIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'
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
import type { Server } from '@/types/api'

const auth = useAuthStore()
const servers = ref<Server[]>([])
const conns = ref<DBConn[]>([])
const selectedConn = ref<DBConn | null>(null)

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
    MessagePlugin.warning('请填写名称并选择服务器')
    return
  }
  addConnLoading.value = true
  try {
    await createConn(connForm.value)
    MessagePlugin.success('连接已创建')
    addConnVisible.value = false
    await loadConns()
  } catch (e: any) {
    MessagePlugin.error(e?.response?.data?.msg ?? '创建失败')
  } finally { addConnLoading.value = false }
}

async function deleteConnItem(c: DBConn) {
  await deleteConn(c.id)
  if (selectedConn.value?.id === c.id) selectedConn.value = null
  await loadConns()
}

function selectConn(c: DBConn) {
  selectedConn.value = c
  mysqlTab.value = 'databases'
  redisTab.value = 'info'
  if (c.type === 'mysql') loadDatabases()
  else loadRedisInfo()
}

async function testConnection() {
  if (!selectedConn.value) return
  try {
    const res = await testConn(selectedConn.value.id)
    MessagePlugin.success(`连接成功: ${res?.output ?? 'OK'}`)
  } catch (e: any) {
    MessagePlugin.error(e?.response?.data?.msg ?? '连接失败')
  }
}

async function loadConns() { conns.value = await listConns() }

// ── MySQL ────────────────────────────────────────────────────────
const mysqlTab = ref('databases')
const dbLoading = ref(false)
const databases = ref<string[]>([])
const databaseRows = computed(() => databases.value.map(name => ({ name })))
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

const dbTableColumns = [
  { colKey: 'name', title: '数据库名', minWidth: 200 },
  { colKey: 'operations', title: '操作', width: 160, fixed: 'right' as const },
]
const userColumns = [
  { colKey: 'user', title: '用户名', minWidth: 150 },
  { colKey: 'host', title: 'Host', width: 160 },
]
const statusColumns = [
  { colKey: 'key', title: '变量名', width: 300, ellipsis: true },
  { colKey: 'val', title: '值', minWidth: 200, ellipsis: true },
]

const queryRowsData = computed(() => {
  if (!queryResult.value) return []
  return queryResult.value.rows.map((row, idx) => {
    const obj: Record<string, string> = { _idx: String(idx) }
    queryResult.value!.columns.forEach((col, i) => { obj[col] = row[i] })
    return obj
  })
})
const queryColumnsData = computed(() =>
  queryResult.value?.columns.map(col => ({ colKey: col, title: col, minWidth: 120, ellipsis: true })) ?? []
)

watch(mysqlTab, async (tab) => {
  if (!selectedConn.value) return
  if (tab === 'databases') loadDatabases()
  if (tab === 'users') loadUsers()
  if (tab === 'status') loadStatus()
  if (tab === 'query') { await nextTick(); initSqlEditor() }
})

function initSqlEditor() {
  if (!sqlEditorEl.value || sqlEditor) return
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
    MessagePlugin.success('数据库已创建')
    createDbVisible.value = false
    await loadDatabases()
  } catch (e: any) { MessagePlugin.error(e?.response?.data?.msg ?? '创建失败') }
  finally { createDbLoading.value = false }
}

async function dropDatabase(dbname: string) {
  if (!selectedConn.value) return
  try {
    await mysqlDropDatabase(selectedConn.value.id, dbname)
    MessagePlugin.success('已删除')
    await loadDatabases()
  } catch (e: any) { MessagePlugin.error(e?.response?.data?.msg ?? '删除失败') }
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
    MessagePlugin.warning('请填写用户名和密码')
    return
  }
  createUserLoading.value = true
  try {
    await mysqlCreateUser(selectedConn.value.id, createUserForm.value)
    MessagePlugin.success('用户已创建')
    createUserVisible.value = false
    await loadUsers()
  } catch (e: any) { MessagePlugin.error(e?.response?.data?.msg ?? '创建失败') }
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
    MessagePlugin.success('FLUSHDB 已执行')
    await loadKeys()
  } catch (e: any) { MessagePlugin.error(e?.response?.data?.msg ?? '失败') }
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
/* 整体两栏布局 */
.db-layout {
  display: flex;
  gap: 16px;
  padding: 20px 24px;
  min-height: calc(100vh - 60px);
  align-items: flex-start;
}

/* 左侧边栏 */
.db-sidebar {
  width: 220px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  padding: 0;
  overflow: hidden;
}
.db-sidebar .section-title {
  padding: 12px 14px;
  font-size: 13px;
  border-bottom: 1px solid var(--sh-border);
}

.sidebar-list {
  flex: 1;
  overflow-y: auto;
}

.conn-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  cursor: pointer;
  border-bottom: 1px solid #f5f5f5;
  transition: background 0.15s;
  position: relative;
}
.conn-item:hover { background: #f7f8fa; }
.conn-item.active {
  background: #EFF4FF;
  border-left: 3px solid var(--sh-blue);
}
.conn-item.active .conn-name { color: var(--sh-blue); }

.conn-item__icon {
  width: 28px;
  height: 28px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 14px;
  flex-shrink: 0;
}
.conn-item__icon--mysql { background: #EFF4FF; color: var(--sh-blue); }
.conn-item__icon--redis { background: #FFF3E8; color: var(--sh-orange); }

.conn-info { flex: 1; min-width: 0; }
.conn-name {
  font-size: 13px;
  font-weight: 500;
  color: var(--sh-text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.conn-meta { font-size: 11px; color: var(--sh-text-secondary); margin-top: 1px; }

.conn-type-badge { display: none; }
.conn-delete-btn { opacity: 0; transition: opacity 0.15s; }
.conn-item:hover .conn-delete-btn { opacity: 1; }

/* 右侧主面板 */
.db-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.db-main-header {
  padding: 0;
}
.db-main-header .section-title {
  border-bottom: none;
  padding: 12px 16px;
}
.db-main-title {
  display: flex;
  align-items: center;
  font-size: 15px;
  font-weight: 600;
}

.db-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 300px;
}

/* Tabs 内容 */
.tab-content { padding: 16px 0; }
.tab-toolbar { display: flex; gap: 8px; margin-bottom: 12px; align-items: center; }
.query-toolbar { display: flex; gap: 8px; margin-bottom: 8px; align-items: center; }

.sql-editor {
  height: 200px;
  overflow: auto;
  border: 1px solid var(--sh-border);
  border-radius: 4px;
  margin-bottom: 12px;
}
:deep(.cm-editor) { height: 100%; }
:deep(.cm-scroller) { overflow: auto; }

.query-result { margin-top: 8px; }
.result-meta { font-size: 12px; color: var(--sh-text-secondary); margin-top: 6px; }
.query-error {
  color: var(--sh-red);
  background: #fff0f0;
  padding: 8px 12px;
  border-radius: 4px;
  font-size: 13px;
  margin-top: 8px;
}

/* Redis */
.redis-info-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
  gap: 6px;
}
.info-item {
  display: flex;
  gap: 8px;
  padding: 5px 10px;
  background: #f7f8fa;
  border-radius: 4px;
  font-size: 12px;
}
.info-key { color: var(--sh-text-secondary); min-width: 180px; flex-shrink: 0; }
.info-val { color: var(--sh-text-primary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

.keys-layout { display: flex; gap: 12px; height: 400px; }
.keys-list {
  width: 260px;
  flex-shrink: 0;
  overflow-y: auto;
  border: 1px solid var(--sh-border);
  border-radius: 4px;
}
.key-item {
  padding: 7px 10px;
  font-size: 12px;
  font-family: monospace;
  cursor: pointer;
  border-bottom: 1px solid #f5f5f5;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--sh-text-primary);
}
.key-item:hover { background: #f7f8fa; }
.key-item.active { background: #EFF4FF; color: var(--sh-blue); }

.key-detail {
  flex: 1;
  border: 1px solid var(--sh-border);
  border-radius: 4px;
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  overflow: auto;
}
.key-detail-header { display: flex; align-items: center; gap: 8px; }
.key-ttl { font-size: 12px; color: var(--sh-text-secondary); }
.key-value {
  flex: 1;
  background: #1a2332;
  color: #a0f0a0;
  padding: 10px;
  border-radius: 4px;
  font-size: 12px;
  font-family: monospace;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-all;
  margin: 0;
}
</style>
