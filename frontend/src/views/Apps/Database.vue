<template>
  <div class="database-page">
    <template v-if="conn">
      <el-descriptions :column="2" border class="desc-block">
        <el-descriptions-item label="名称">{{ conn.name }}</el-descriptions-item>
        <el-descriptions-item label="类型"><el-tag size="small">{{ conn.type.toUpperCase() }}</el-tag></el-descriptions-item>
        <el-descriptions-item label="主机">{{ conn.host }}:{{ conn.port }}</el-descriptions-item>
        <el-descriptions-item label="用户">{{ conn.username || '—' }}</el-descriptions-item>
        <el-descriptions-item label="数据库">{{ conn.database || '—' }}</el-descriptions-item>
        <el-descriptions-item label="状态">
          <el-tag :type="testResult === 'ok' ? 'success' : testResult === 'fail' ? 'danger' : 'info'" size="small">
            {{ testResult === 'ok' ? '连接正常' : testResult === 'fail' ? '连接失败' : '未检测' }}
          </el-tag>
        </el-descriptions-item>
      </el-descriptions>
      <el-button :loading="testing" @click="doTest" style="margin-bottom:16px">连接测试</el-button>

      <template v-if="conn.type === 'mysql'">
        <el-divider>数据库列表</el-divider>
        <div class="db-toolbar">
          <el-button size="small" type="primary" @click="openCreateDB">新建</el-button>
          <el-button size="small" @click="loadDBs">刷新</el-button>
        </div>
        <el-table :data="databases" v-loading="dbLoading" style="width:100%">
          <el-table-column label="数据库名" prop="name" min-width="200" />
          <el-table-column label="操作" width="120" fixed="right">
            <template #default="{ row }">
              <el-popconfirm :title="`确认删除数据库 ${row.name}？此操作不可恢复`" @confirm="dropDB(row.name)">
                <template #reference>
                  <el-button size="small" type="danger">删除</el-button>
                </template>
              </el-popconfirm>
            </template>
          </el-table-column>
        </el-table>
      </template>

      <template v-if="conn.type === 'redis'">
        <el-divider>Redis 状态</el-divider>
        <el-button size="small" @click="loadRedisInfo" style="margin-bottom:12px">刷新</el-button>
        <el-table :data="redisInfoRows" v-loading="redisLoading" style="width:100%">
          <el-table-column label="键" prop="key" width="220" />
          <el-table-column label="值" prop="val" min-width="300" show-overflow-tooltip />
        </el-table>
      </template>
    </template>
    <el-empty v-else-if="!loading" description="该应用未关联数据库连接，请先在应用设置中配置 db_conn_id" />

    <el-dialog v-model="createDBVisible" title="新建数据库" width="400px">
      <el-form :model="createDBForm" label-width="80px">
        <el-form-item label="库名"><el-input v-model="createDBForm.name" placeholder="my_database" /></el-form-item>
        <el-form-item label="字符集"><el-input v-model="createDBForm.charset" placeholder="utf8mb4（默认）" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createDBVisible = false">取消</el-button>
        <el-button type="primary" @click="confirmCreateDB">创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useAppStore } from '@/stores/app'
import { listConns, testConn, mysqlDatabases, mysqlCreateDatabase, mysqlDropDatabase, redisInfo } from '@/api/database'
import type { DBConn } from '@/api/database'

const route = useRoute()
const appStore = useAppStore()
const appId = computed(() => Number(route.params.appId))
const app = computed(() => appStore.getById(appId.value))

const conn = ref<DBConn | null>(null)
const loading = ref(false)
const testing = ref(false)
const testResult = ref<'ok' | 'fail' | ''>('')

const databases = ref<{ name: string }[]>([])
const dbLoading = ref(false)

const redisInfoRows = ref<{ key: string; val: string }[]>([])
const redisLoading = ref(false)

const createDBVisible = ref(false)
const createDBForm = ref({ name: '', charset: '' })

async function loadConn() {
  if (!app.value?.db_conn_id) return
  loading.value = true
  try {
    const all = await listConns()
    conn.value = (all as unknown as DBConn[]).find(c => c.id === app.value!.db_conn_id) ?? null
  } finally { loading.value = false }
}

async function doTest() {
  if (!conn.value) return
  testing.value = true
  try { await testConn(conn.value.id); testResult.value = 'ok'; ElMessage.success('连接正常') }
  catch { testResult.value = 'fail'; ElMessage.error('连接失败') }
  finally { testing.value = false }
}

async function loadDBs() {
  if (!conn.value) return
  dbLoading.value = true
  try {
    const dbs = await mysqlDatabases(conn.value.id)
    databases.value = dbs.map(name => ({ name }))
  } catch { ElMessage.error('加载失败') }
  finally { dbLoading.value = false }
}

function openCreateDB() { createDBForm.value = { name: '', charset: '' }; createDBVisible.value = true }

async function confirmCreateDB() {
  if (!conn.value || !createDBForm.value.name) return
  try {
    await mysqlCreateDatabase(conn.value.id, createDBForm.value.name, createDBForm.value.charset || undefined)
    ElMessage.success('已创建'); createDBVisible.value = false; await loadDBs()
  } catch { ElMessage.error('创建失败') }
}

async function dropDB(name: string) {
  if (!conn.value) return
  try { await mysqlDropDatabase(conn.value.id, name); ElMessage.success('已删除'); await loadDBs() }
  catch { ElMessage.error('删除失败') }
}

async function loadRedisInfo() {
  if (!conn.value) return
  redisLoading.value = true
  try {
    const info = await redisInfo(conn.value.id)
    redisInfoRows.value = Object.entries(info).map(([key, val]) => ({ key, val }))
  } catch { ElMessage.error('加载失败') }
  finally { redisLoading.value = false }
}

onMounted(async () => {
  if (!appStore.apps.length) await appStore.fetch()
  await loadConn()
  if (conn.value?.type === 'mysql') await loadDBs()
  if (conn.value?.type === 'redis') await loadRedisInfo()
})
</script>

<style scoped>
.database-page { padding: 4px 0; }
.desc-block { margin-bottom: 16px; }
.db-toolbar { display: flex; gap: 8px; margin-bottom: 12px; }
</style>
