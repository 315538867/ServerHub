<template>
  <div class="page-container">
    <template v-if="conn">
      <!-- 连接信息 -->
      <div class="section-block">
        <div class="section-title">
          <span class="title-text">连接信息</span>
          <t-button size="small" :loading="testing" @click="doTest">连接测试</t-button>
        </div>
        <div class="desc-wrap">
          <t-descriptions :column="2">
            <t-descriptions-item label="名称">{{ conn.name }}</t-descriptions-item>
            <t-descriptions-item label="类型"><t-tag size="small" variant="light">{{ conn.type.toUpperCase() }}</t-tag></t-descriptions-item>
            <t-descriptions-item label="主机">{{ conn.host }}:{{ conn.port }}</t-descriptions-item>
            <t-descriptions-item label="用户">{{ conn.username || '—' }}</t-descriptions-item>
            <t-descriptions-item label="数据库">{{ conn.database || '—' }}</t-descriptions-item>
            <t-descriptions-item label="连接状态">
              <t-tag :theme="testTheme" variant="light" size="small">
                {{ testResult === 'ok' ? '连接正常' : testResult === 'fail' ? '连接失败' : '未检测' }}
              </t-tag>
            </t-descriptions-item>
          </t-descriptions>
        </div>
      </div>

      <!-- MySQL 数据库列表 -->
      <template v-if="conn.type === 'mysql'">
        <div class="section-block">
          <div class="section-title">
            <span class="title-text">数据库列表</span>
            <t-space size="small">
              <t-button size="small" theme="primary" @click="openCreateDB">新建</t-button>
              <t-button size="small" variant="outline" @click="loadDBs">刷新</t-button>
            </t-space>
          </div>
          <div class="table-wrap">
            <t-table :data="databases" :columns="dbColumns" :loading="dbLoading" row-key="name" stripe size="small">
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
            <t-button size="small" variant="outline" @click="loadRedisInfo">刷新</t-button>
          </div>
          <div class="table-wrap">
            <t-table :data="redisInfoRows" :columns="redisColumns" :loading="redisLoading" row-key="key" stripe size="small" />
          </div>
        </div>
      </template>
    </template>
    <div v-else-if="!loading" class="section-block empty-block">
      <t-empty description="该应用未关联数据库连接，请先在应用设置中配置 db_conn_id" />
    </div>

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
.desc-wrap {
  padding: 16px 20px 20px;
}
:deep(.t-descriptions__label) {
  color: var(--sh-text-secondary);
  font-size: 13px;
  width: 80px;
}
:deep(.t-descriptions__content) {
  font-size: 13px;
}
.table-wrap {
  padding: 0 20px 16px;
}
:deep(.t-table td) {
  font-size: 13px;
}
.empty-block {
  padding: 40px 20px;
  display: flex;
  justify-content: center;
}
</style>
