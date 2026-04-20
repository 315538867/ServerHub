<template>
  <div class="database-page">
    <template v-if="conn">
      <t-descriptions :column="2" bordered style="margin-bottom:16px">
        <t-descriptions-item label="名称">{{ conn.name }}</t-descriptions-item>
        <t-descriptions-item label="类型"><t-tag size="small" variant="light">{{ conn.type.toUpperCase() }}</t-tag></t-descriptions-item>
        <t-descriptions-item label="主机">{{ conn.host }}:{{ conn.port }}</t-descriptions-item>
        <t-descriptions-item label="用户">{{ conn.username || '—' }}</t-descriptions-item>
        <t-descriptions-item label="数据库">{{ conn.database || '—' }}</t-descriptions-item>
        <t-descriptions-item label="状态">
          <t-tag :theme="testTheme" variant="light" size="small">
            {{ testResult === 'ok' ? '连接正常' : testResult === 'fail' ? '连接失败' : '未检测' }}
          </t-tag>
        </t-descriptions-item>
      </t-descriptions>
      <t-button :loading="testing" @click="doTest" style="margin-bottom:16px">连接测试</t-button>

      <template v-if="conn.type === 'mysql'">
        <div class="section-divider"><span>数据库列表</span></div>
        <div class="db-toolbar">
          <t-button size="small" theme="primary" @click="openCreateDB">新建</t-button>
          <t-button size="small" variant="outline" @click="loadDBs">刷新</t-button>
        </div>
        <t-table :data="databases" :columns="dbColumns" :loading="dbLoading" row-key="name" stripe>
          <template #operations="{ row }">
            <t-popconfirm :content="`确认删除数据库 ${row.name}？此操作不可恢复`" @confirm="dropDB(row.name)">
              <t-button theme="danger" size="small" variant="text">删除</t-button>
            </t-popconfirm>
          </template>
        </t-table>
      </template>

      <template v-if="conn.type === 'redis'">
        <div class="section-divider"><span>Redis 状态</span></div>
        <t-button size="small" variant="outline" @click="loadRedisInfo" style="margin-bottom:12px">刷新</t-button>
        <t-table :data="redisInfoRows" :columns="redisColumns" :loading="redisLoading" row-key="key" stripe />
      </template>
    </template>
    <t-empty v-else-if="!loading" description="该应用未关联数据库连接，请先在应用设置中配置 db_conn_id" />

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
import { useRoute } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
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
  try { await testConn(conn.value.id); testResult.value = 'ok'; MessagePlugin.success('连接正常') }
  catch { testResult.value = 'fail'; MessagePlugin.error('连接失败') }
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

onMounted(async () => {
  if (!appStore.apps.length) await appStore.fetch()
  await loadConn()
  if (conn.value?.type === 'mysql') await loadDBs()
  if (conn.value?.type === 'redis') await loadRedisInfo()
})
</script>

<style scoped>
.database-page { padding: 4px 0; }
.section-divider {
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 20px 0 12px;
  font-size: 13px;
  font-weight: 600;
  color: var(--td-text-color-secondary);
}
.section-divider::before, .section-divider::after {
  content: '';
  flex: 1;
  height: 1px;
  background: var(--td-component-border);
}
.db-toolbar { display: flex; gap: 8px; margin-bottom: 12px; }
</style>
