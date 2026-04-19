<template>
  <div class="db-page">
    <!-- Left sidebar: connection list -->
    <div class="db-sidebar">
      <div class="sidebar-header">
        <span class="sidebar-title">数据库连接</span>
        <el-button :icon="Plus" circle size="small" @click="openAddConn" />
      </div>
      <el-scrollbar>
        <div
          v-for="c in conns"
          :key="c.id"
          class="conn-item"
          :class="{ active: selectedConn?.id === c.id }"
          @click="selectConn(c)"
        >
          <el-icon class="conn-icon"><component :is="c.type === 'redis' ? 'Star' : 'Grid'" /></el-icon>
          <div class="conn-info">
            <div class="conn-name">{{ c.name }}</div>
            <div class="conn-meta">{{ c.type.toUpperCase() }} · {{ c.host }}:{{ c.port }}</div>
          </div>
          <el-button :icon="Delete" circle size="small" type="danger" plain @click.stop="deleteConnItem(c)" />
        </div>
        <el-empty v-if="conns.length === 0" description="暂无连接" :image-size="60" />
      </el-scrollbar>
    </div>

    <!-- Right content -->
    <div class="db-content" v-if="selectedConn">
      <div class="content-header">
        <span class="content-title">{{ selectedConn.name }}</span>
        <el-tag :type="selectedConn.type === 'redis' ? 'warning' : 'primary'" size="small">{{ selectedConn.type.toUpperCase() }}</el-tag>
        <el-button size="small" @click="testConnection">测试连接</el-button>
      </div>

      <!-- MySQL view -->
      <template v-if="selectedConn.type === 'mysql'">
        <el-tabs v-model="mysqlTab">
          <el-tab-pane label="数据库" name="databases">
            <div class="tab-toolbar">
              <el-button type="primary" size="small" @click="openCreateDb">建库</el-button>
              <el-button size="small" :loading="dbLoading" @click="loadDatabases">刷新</el-button>
            </div>
            <el-table :data="databases" v-loading="dbLoading" size="small">
              <el-table-column label="数据库名" prop="" min-width="200">
                <template #default="{ row }">{{ row }}</template>
              </el-table-column>
              <el-table-column label="操作" width="160">
                <template #default="{ row }">
                  <el-button size="small" @click="exportDatabase(row)">导出</el-button>
                  <el-popconfirm :title="`确认删除数据库 ${row}？不可恢复！`" @confirm="dropDatabase(row)">
                    <template #reference>
                      <el-button size="small" type="danger">删除</el-button>
                    </template>
                  </el-popconfirm>
                </template>
              </el-table-column>
            </el-table>
          </el-tab-pane>

          <el-tab-pane label="用户" name="users">
            <div class="tab-toolbar">
              <el-button type="primary" size="small" @click="openCreateUser">添加用户</el-button>
              <el-button size="small" :loading="userLoading" @click="loadUsers">刷新</el-button>
            </div>
            <el-table :data="users" v-loading="userLoading" size="small">
              <el-table-column label="用户名" prop="user" min-width="150" />
              <el-table-column label="Host" prop="host" width="160" />
            </el-table>
          </el-tab-pane>

          <el-tab-pane label="SQL 执行器" name="query">
            <div class="query-toolbar">
              <el-select v-model="queryDb" placeholder="选择数据库" size="small" style="width:180px" clearable>
                <el-option v-for="d in databases" :key="d" :label="d" :value="d" />
              </el-select>
              <el-button type="primary" size="small" :loading="queryLoading" @click="runQuery">执行</el-button>
            </div>
            <div ref="sqlEditorEl" class="sql-editor" />
            <div v-if="queryResult" class="query-result">
              <el-table :data="queryResult.rows" size="small" style="width:100%" max-height="300">
                <el-table-column
                  v-for="(col, i) in queryResult.columns"
                  :key="i"
                  :label="col"
                  :prop="String(i)"
                  show-overflow-tooltip
                >
                  <template #default="{ row }">{{ row[i] }}</template>
                </el-table-column>
              </el-table>
              <div class="result-meta">共 {{ queryResult.rows.length }} 行</div>
            </div>
            <div v-if="queryError" class="query-error">{{ queryError }}</div>
          </el-tab-pane>

          <el-tab-pane label="状态" name="status">
            <el-button size="small" :loading="statusLoading" @click="loadStatus" style="margin-bottom:12px">刷新</el-button>
            <el-table :data="statusRows" v-loading="statusLoading" size="small" max-height="500">
              <el-table-column label="变量名" prop="0" width="300" show-overflow-tooltip />
              <el-table-column label="值" prop="1" show-overflow-tooltip />
            </el-table>
          </el-tab-pane>
        </el-tabs>
      </template>

      <!-- Redis view -->
      <template v-else-if="selectedConn.type === 'redis'">
        <el-tabs v-model="redisTab">
          <el-tab-pane label="状态" name="info">
            <el-button size="small" :loading="infoLoading" @click="loadRedisInfo" style="margin-bottom:12px">刷新</el-button>
            <div class="redis-info-grid">
              <div v-for="(val, key) in redisInfo" :key="key" class="info-item">
                <span class="info-key">{{ key }}</span>
                <span class="info-val">{{ val }}</span>
              </div>
            </div>
          </el-tab-pane>

          <el-tab-pane label="Key 浏览" name="keys">
            <div class="tab-toolbar">
              <el-input v-model="keyPattern" placeholder="搜索 Pattern（默认 *）" size="small" style="width:220px" />
              <el-button size="small" :loading="keysLoading" @click="loadKeys">搜索</el-button>
              <el-popconfirm title="确认 FLUSHDB？所有数据将被清空！" @confirm="doFlushDB">
                <template #reference>
                  <el-button size="small" type="danger">FLUSHDB</el-button>
                </template>
              </el-popconfirm>
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
                  <el-tag size="small">{{ keyDetail.type }}</el-tag>
                  <span class="key-ttl">TTL: {{ keyDetail.ttl }}s</span>
                  <el-button size="small" type="danger" @click="deleteKey(selectedKey!)">删除</el-button>
                </div>
                <pre class="key-value">{{ keyDetail.value }}</pre>
              </div>
            </div>
          </el-tab-pane>
        </el-tabs>
      </template>
    </div>

    <div class="db-content db-empty" v-else>
      <el-empty description="选择或创建一个数据库连接" />
    </div>

    <!-- Add connection dialog -->
    <el-dialog v-model="addConnVisible" title="添加数据库连接" width="480px">
      <el-form :model="connForm" label-width="80px" size="small">
        <el-form-item label="名称">
          <el-input v-model="connForm.name" placeholder="My MySQL" />
        </el-form-item>
        <el-form-item label="服务器">
          <el-select v-model="connForm.server_id" placeholder="选择服务器" style="width:100%">
            <el-option v-for="s in servers" :key="s.id" :label="`${s.name} (${s.host})`" :value="s.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="类型">
          <el-radio-group v-model="connForm.type">
            <el-radio value="mysql">MySQL</el-radio>
            <el-radio value="redis">Redis</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="Host">
          <el-input v-model="connForm.host" placeholder="127.0.0.1" />
        </el-form-item>
        <el-form-item label="端口">
          <el-input-number v-model="connForm.port" :min="1" :max="65535" style="width:100%" />
        </el-form-item>
        <el-form-item label="用户名" v-if="connForm.type === 'mysql'">
          <el-input v-model="connForm.username" placeholder="root" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="connForm.password" type="password" show-password />
        </el-form-item>
        <el-form-item label="默认库" v-if="connForm.type === 'mysql'">
          <el-input v-model="connForm.database" placeholder="可选" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="addConnVisible = false">取消</el-button>
        <el-button type="primary" :loading="addConnLoading" @click="confirmAddConn">保存</el-button>
      </template>
    </el-dialog>

    <!-- Create database dialog -->
    <el-dialog v-model="createDbVisible" title="创建数据库" width="400px">
      <el-form :model="createDbForm" label-width="80px" size="small">
        <el-form-item label="库名">
          <el-input v-model="createDbForm.name" placeholder="mydb" />
        </el-form-item>
        <el-form-item label="字符集">
          <el-select v-model="createDbForm.charset" style="width:100%">
            <el-option label="utf8mb4" value="utf8mb4" />
            <el-option label="utf8" value="utf8" />
            <el-option label="latin1" value="latin1" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createDbVisible = false">取消</el-button>
        <el-button type="primary" :loading="createDbLoading" @click="confirmCreateDb">创建</el-button>
      </template>
    </el-dialog>

    <!-- Create user dialog -->
    <el-dialog v-model="createUserVisible" title="添加用户" width="400px">
      <el-form :model="createUserForm" label-width="80px" size="small">
        <el-form-item label="用户名">
          <el-input v-model="createUserForm.user" placeholder="appuser" />
        </el-form-item>
        <el-form-item label="Host">
          <el-input v-model="createUserForm.host" placeholder="%" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="createUserForm.password" type="password" show-password />
        </el-form-item>
        <el-form-item label="授权库">
          <el-input v-model="createUserForm.database" placeholder="留空为 *.*" />
        </el-form-item>
        <el-form-item label="权限">
          <el-select v-model="createUserForm.grant" style="width:100%">
            <el-option label="ALL PRIVILEGES" value="ALL PRIVILEGES" />
            <el-option label="SELECT" value="SELECT" />
            <el-option label="SELECT, INSERT, UPDATE, DELETE" value="SELECT, INSERT, UPDATE, DELETE" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createUserVisible = false">取消</el-button>
        <el-button type="primary" :loading="createUserLoading" @click="confirmCreateUser">创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, nextTick, onBeforeUnmount } from 'vue'
import { Plus, Delete } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
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
const connForm = ref({ name: '', server_id: 0, type: 'mysql' as 'mysql'|'redis', host: '127.0.0.1', port: 3306, username: 'root', password: '', database: '' })

watch(() => connForm.value.type, (t) => {
  connForm.value.port = t === 'redis' ? 6379 : 3306
})

function openAddConn() {
  connForm.value = { name: '', server_id: servers.value[0]?.id ?? 0, type: 'mysql', host: '127.0.0.1', port: 3306, username: 'root', password: '', database: '' }
  addConnVisible.value = true
}

async function confirmAddConn() {
  if (!connForm.value.name || !connForm.value.server_id) {
    ElMessage.warning('请填写名称并选择服务器')
    return
  }
  addConnLoading.value = true
  try {
    await createConn(connForm.value)
    ElMessage.success('连接已创建')
    addConnVisible.value = false
    await loadConns()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.msg ?? '创建失败')
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
  if (c.type === 'mysql') {
    loadDatabases()
  } else {
    loadRedisInfo()
  }
}

async function testConnection() {
  if (!selectedConn.value) return
  try {
    const res = await testConn(selectedConn.value.id)
    ElMessage.success(`连接成功: ${res?.output ?? 'OK'}`)
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.msg ?? '连接失败')
  }
}

async function loadConns() {
  conns.value = await listConns()
}

// ── MySQL ────────────────────────────────────────────────────────
const mysqlTab = ref('databases')
const dbLoading = ref(false)
const databases = ref<string[]>([])
const userLoading = ref(false)
const users = ref<Array<{ user: string; host: string }>>([])
const statusLoading = ref(false)
const statusRows = ref<string[][]>([])
const queryLoading = ref(false)
const queryDb = ref('')
const queryResult = ref<{ columns: string[]; rows: string[][] } | null>(null)
const queryError = ref('')
const sqlEditorEl = ref<HTMLDivElement>()
let sqlEditor: EditorView | null = null

watch(mysqlTab, async (tab) => {
  if (!selectedConn.value) return
  if (tab === 'databases') loadDatabases()
  if (tab === 'users') loadUsers()
  if (tab === 'status') loadStatus()
  if (tab === 'query') {
    await nextTick()
    initSqlEditor()
  }
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
    statusRows.value = res.rows
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

// Create database
const createDbVisible = ref(false)
const createDbLoading = ref(false)
const createDbForm = ref({ name: '', charset: 'utf8mb4' })

function openCreateDb() { createDbForm.value = { name: '', charset: 'utf8mb4' }; createDbVisible.value = true }

async function confirmCreateDb() {
  if (!selectedConn.value || !createDbForm.value.name) return
  createDbLoading.value = true
  try {
    await mysqlCreateDatabase(selectedConn.value.id, createDbForm.value.name, createDbForm.value.charset)
    ElMessage.success('数据库已创建')
    createDbVisible.value = false
    await loadDatabases()
  } catch (e: any) { ElMessage.error(e?.response?.data?.msg ?? '创建失败') }
  finally { createDbLoading.value = false }
}

async function dropDatabase(dbname: string) {
  if (!selectedConn.value) return
  try {
    await mysqlDropDatabase(selectedConn.value.id, dbname)
    ElMessage.success('已删除')
    await loadDatabases()
  } catch (e: any) { ElMessage.error(e?.response?.data?.msg ?? '删除失败') }
}

function exportDatabase(dbname: string) {
  if (!selectedConn.value) return
  const url = mysqlExportUrl(selectedConn.value.id, dbname, auth.token)
  window.open(url, '_blank')
}

// Create user
const createUserVisible = ref(false)
const createUserLoading = ref(false)
const createUserForm = ref({ user: '', host: '%', password: '', database: '', grant: 'ALL PRIVILEGES' })

function openCreateUser() { createUserForm.value = { user: '', host: '%', password: '', database: '', grant: 'ALL PRIVILEGES' }; createUserVisible.value = true }

async function confirmCreateUser() {
  if (!selectedConn.value || !createUserForm.value.user || !createUserForm.value.password) {
    ElMessage.warning('请填写用户名和密码')
    return
  }
  createUserLoading.value = true
  try {
    await mysqlCreateUser(selectedConn.value.id, createUserForm.value)
    ElMessage.success('用户已创建')
    createUserVisible.value = false
    await loadUsers()
  } catch (e: any) { ElMessage.error(e?.response?.data?.msg ?? '创建失败') }
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
    ElMessage.success('FLUSHDB 已执行')
    await loadKeys()
  } catch (e: any) { ElMessage.error(e?.response?.data?.msg ?? '失败') }
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
.db-page {
  display: flex;
  height: 100%;
  min-height: calc(100vh - 120px);
  gap: 0;
}
.db-sidebar {
  width: 240px;
  flex-shrink: 0;
  border-right: 1px solid #e4e7ed;
  display: flex;
  flex-direction: column;
  background: #fafafa;
}
.sidebar-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 14px;
  border-bottom: 1px solid #e4e7ed;
}
.sidebar-title { font-weight: 600; font-size: 13px; }
.conn-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  cursor: pointer;
  border-bottom: 1px solid #f0f0f0;
  transition: background 0.15s;
}
.conn-item:hover { background: #f0f5ff; }
.conn-item.active { background: #ecf5ff; }
.conn-icon { font-size: 16px; color: #409eff; flex-shrink: 0; }
.conn-info { flex: 1; overflow: hidden; }
.conn-name { font-size: 13px; font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.conn-meta { font-size: 11px; color: #909399; }
.db-content {
  flex: 1;
  padding: 20px;
  overflow: auto;
}
.db-empty {
  display: flex;
  align-items: center;
  justify-content: center;
}
.content-header {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 16px;
}
.content-title { font-size: 16px; font-weight: 600; }
.tab-toolbar { display: flex; gap: 8px; margin-bottom: 12px; }
.query-toolbar { display: flex; gap: 8px; margin-bottom: 8px; align-items: center; }
.sql-editor {
  height: 200px;
  overflow: auto;
  border: 1px solid #e4e7ed;
  border-radius: 4px;
  margin-bottom: 12px;
}
:deep(.cm-editor) { height: 100%; }
:deep(.cm-scroller) { overflow: auto; }
.query-result { margin-top: 8px; }
.result-meta { font-size: 12px; color: #909399; margin-top: 6px; }
.query-error {
  color: #f56c6c;
  background: #fef0f0;
  padding: 8px 12px;
  border-radius: 4px;
  font-size: 13px;
  margin-top: 8px;
}
.redis-info-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
  gap: 6px;
}
.info-item {
  display: flex;
  gap: 8px;
  padding: 4px 8px;
  background: #f5f7fa;
  border-radius: 4px;
  font-size: 12px;
}
.info-key { color: #606266; min-width: 180px; flex-shrink: 0; }
.info-val { color: #303133; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.keys-layout { display: flex; gap: 12px; height: 400px; }
.keys-list {
  width: 260px;
  flex-shrink: 0;
  overflow-y: auto;
  border: 1px solid #e4e7ed;
  border-radius: 4px;
}
.key-item {
  padding: 6px 10px;
  font-size: 12px;
  font-family: monospace;
  cursor: pointer;
  border-bottom: 1px solid #f0f0f0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.key-item:hover { background: #f0f5ff; }
.key-item.active { background: #ecf5ff; color: #409eff; }
.key-detail {
  flex: 1;
  border: 1px solid #e4e7ed;
  border-radius: 4px;
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  overflow: auto;
}
.key-detail-header { display: flex; align-items: center; gap: 8px; }
.key-ttl { font-size: 12px; color: #909399; }
.key-value {
  flex: 1;
  background: #1a1a2e;
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
