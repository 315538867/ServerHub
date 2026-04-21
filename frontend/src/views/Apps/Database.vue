<template>
  <div class="db-page">
    <template v-if="conn">
      <UiSection>
        <template #title>
          <span class="db-title">
            数据库连接
            <UiBadge :tone="testTone">
              <span class="db-dot" :class="testResult || 'unknown'" />
              {{ testResult === 'ok' ? '在线' : testResult === 'fail' ? '离线' : '未检测' }}
            </UiBadge>
          </span>
        </template>
        <template #extra>
          <UiButton variant="secondary" size="sm" :loading="testing" @click="doTest(false)">连接测试</UiButton>
          <UiButton variant="secondary" size="sm" @click="goGlobalDB">管理全部连接</UiButton>
          <UiButton v-if="canUnbind" variant="danger" size="sm" @click="unbindConn">解除绑定</UiButton>
        </template>
        <UiCard padding="md">
          <div class="db-desc">
            <div class="db-desc__cell"><span class="lbl">名称</span><span class="val">{{ conn.name }}</span></div>
            <div class="db-desc__cell"><span class="lbl">类型</span><UiBadge tone="info">{{ conn.type.toUpperCase() }}</UiBadge></div>
            <div class="db-desc__cell"><span class="lbl">主机</span><code class="mono">{{ conn.host }}:{{ conn.port }}</code></div>
            <div class="db-desc__cell"><span class="lbl">用户</span><span class="val">{{ conn.username || '—' }}</span></div>
            <div class="db-desc__cell"><span class="lbl">数据库</span><span class="val">{{ conn.database || '—' }}</span></div>
            <div class="db-desc__cell"><span class="lbl">服务器</span><span class="val">{{ serverName }}</span></div>
          </div>
        </UiCard>
      </UiSection>

      <UiSection v-if="conn.type === 'mysql'">
        <template #title>
          <span class="db-title">
            数据库列表 <UiBadge tone="neutral">{{ databases.length }}</UiBadge>
          </span>
        </template>
        <template #extra>
          <NInput v-model:value="dbFilter" placeholder="过滤库名" size="small" clearable class="filter-inp">
            <template #prefix><Search :size="14" /></template>
          </NInput>
          <UiButton variant="primary" size="sm" @click="openCreateDB">新建</UiButton>
          <UiButton variant="secondary" size="sm" :loading="dbLoading" @click="loadDBs">刷新</UiButton>
        </template>
        <UiCard padding="none">
          <NDataTable
            :columns="dbColumns"
            :data="filteredDatabases"
            :loading="dbLoading"
            :row-key="(row: { name: string }) => row.name"
            size="small"
            :bordered="false"
          />
        </UiCard>
      </UiSection>

      <UiSection v-if="conn.type === 'redis'" title="Redis 状态">
        <template #extra>
          <NInput v-model:value="redisFilter" placeholder="过滤键" size="small" clearable class="filter-inp">
            <template #prefix><Search :size="14" /></template>
          </NInput>
          <UiButton variant="secondary" size="sm" :loading="redisLoading" @click="loadRedisInfo">刷新</UiButton>
        </template>
        <UiCard padding="none">
          <NDataTable
            :columns="redisColumns"
            :data="filteredRedisRows"
            :loading="redisLoading"
            :row-key="(row: { key: string }) => row.key"
            size="small"
            :bordered="false"
          />
        </UiCard>
      </UiSection>
    </template>

    <div v-else-if="!loading" class="db-empty">
      <div class="db-empty__icon">🗄️</div>
      <div class="db-empty__title">该应用未关联数据库连接</div>
      <div class="db-empty__sub">你可以绑定一个现有连接，或新建一个</div>
      <div class="db-empty__grid">
        <UiCard padding="md">
          <div class="db-empty__card-title">🔗 绑定现有连接</div>
          <div class="db-empty__card-body">
            <NSelect
              v-model:value="bindConnId"
              placeholder="选择连接"
              :options="availableConnOptions"
              size="small"
              clearable
              :disabled="availableConnOptions.length === 0"
            />
            <div v-if="availableConnOptions.length === 0" class="db-empty__hint">
              {{ app?.server_id ? '该服务器下暂无数据库连接' : '应用尚未绑定服务器' }}
            </div>
            <UiButton variant="primary" size="sm" :disabled="!bindConnId" :loading="binding" @click="doBind">绑定</UiButton>
          </div>
        </UiCard>
        <UiCard padding="md">
          <div class="db-empty__card-title">➕ 新建连接</div>
          <div class="db-empty__card-body">
            <div class="db-empty__hint">跳转到「全局数据库」管理页，新建一个连接后回到本 Tab 绑定</div>
            <UiButton variant="secondary" size="sm" @click="goGlobalDB">去新建</UiButton>
          </div>
        </UiCard>
      </div>
    </div>

    <NModal
      v-model:show="createDBVisible"
      preset="card"
      title="新建数据库"
      style="width: 420px"
      :bordered="false"
    >
      <NForm :model="createDBForm" label-placement="left" label-width="80">
        <NFormItem label="库名">
          <NInput v-model:value="createDBForm.name" placeholder="my_database" />
        </NFormItem>
        <NFormItem label="字符集">
          <NInput v-model:value="createDBForm.charset" placeholder="utf8mb4（默认）" />
        </NFormItem>
      </NForm>
      <template #footer>
        <div class="modal-foot">
          <UiButton variant="secondary" size="sm" @click="createDBVisible = false">取消</UiButton>
          <UiButton variant="primary" size="sm" @click="confirmCreateDB">创建</UiButton>
        </div>
      </template>
    </NModal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  NInput, NSelect, NDataTable, NModal, NForm, NFormItem, NPopconfirm, useMessage,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { Search } from 'lucide-vue-next'
import { useAppStore } from '@/stores/app'
import { useServerStore } from '@/stores/server'
import { updateApp } from '@/api/application'
import { listConns, testConn, mysqlDatabases, mysqlCreateDatabase, mysqlDropDatabase, redisInfo } from '@/api/database'
import type { DBConn } from '@/api/database'
import UiSection from '@/components/ui/UiSection.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiBadge from '@/components/ui/UiBadge.vue'

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const serverStore = useServerStore()
const message = useMessage()

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

const bindConnId = ref<number | null>(null)
const binding = ref(false)

const dbColumns = computed<DataTableColumns<{ name: string }>>(() => [
  {
    title: '数据库名', key: 'name', minWidth: 200,
    render: (row) => h('code', { class: 'mono' }, row.name),
  },
  {
    title: '操作', key: 'ops', width: 120, fixed: 'right' as const,
    render: (row) => h(NPopconfirm, {
      onPositiveClick: () => dropDB(row.name),
      positiveText: '删除', negativeText: '取消',
    }, {
      trigger: () => h(UiButton, { variant: 'ghost', size: 'sm' },
        () => h('span', { class: 'text-danger' }, '删除')),
      default: () => `确认删除数据库 ${row.name}？此操作不可恢复`,
    }),
  },
])

const redisColumns: DataTableColumns<{ key: string; val: string }> = [
  {
    title: '键', key: 'key', width: 220,
    render: (row) => h('code', { class: 'mono' }, row.key),
  },
  { title: '值', key: 'val', minWidth: 300, ellipsis: { tooltip: true } },
]

const testTone = computed<any>(() => {
  if (testResult.value === 'ok') return 'success'
  if (testResult.value === 'fail') return 'danger'
  return 'neutral'
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
  try { await testConn(conn.value.id); testResult.value = 'ok'; if (!silent) message.success('连接正常') }
  catch { testResult.value = 'fail'; if (!silent) message.error('连接失败') }
  finally { testing.value = false }
}

async function loadDBs() {
  if (!conn.value) return
  dbLoading.value = true
  try {
    const dbs = await mysqlDatabases(conn.value.id)
    databases.value = dbs.map(name => ({ name }))
  } catch { message.error('加载失败') }
  finally { dbLoading.value = false }
}

function openCreateDB() { createDBForm.value = { name: '', charset: '' }; createDBVisible.value = true }

async function confirmCreateDB() {
  if (!conn.value || !createDBForm.value.name) return
  try {
    await mysqlCreateDatabase(conn.value.id, createDBForm.value.name, createDBForm.value.charset || undefined)
    message.success('已创建'); createDBVisible.value = false; await loadDBs()
  } catch { message.error('创建失败') }
}

async function dropDB(name: string) {
  if (!conn.value) return
  try { await mysqlDropDatabase(conn.value.id, name); message.success('已删除'); await loadDBs() }
  catch { message.error('删除失败') }
}

async function loadRedisInfo() {
  if (!conn.value) return
  redisLoading.value = true
  try {
    const info = await redisInfo(conn.value.id)
    redisInfoRows.value = Object.entries(info).map(([key, val]) => ({ key, val }))
  } catch { message.error('加载失败') }
  finally { redisLoading.value = false }
}

async function doBind() {
  if (!bindConnId.value || !app.value) return
  binding.value = true
  try {
    await updateApp(app.value.id, { ...app.value, db_conn_id: bindConnId.value } as any)
    message.success('已绑定')
    await appStore.fetch()
    await loadConn()
    if (conn.value?.type === 'mysql') loadDBs()
    if (conn.value?.type === 'redis') loadRedisInfo()
    doTest(true)
  } catch { message.error('绑定失败') }
  finally { binding.value = false }
}

async function unbindConn() {
  if (!app.value) return
  try {
    await updateApp(app.value.id, { ...app.value, db_conn_id: null } as any)
    message.success('已解除绑定')
    await appStore.fetch()
    conn.value = null
    testResult.value = ''
  } catch { message.error('解除失败') }
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
.db-page { padding: var(--space-6); display: flex; flex-direction: column; gap: var(--space-4); }

.db-title { display: inline-flex; align-items: center; gap: var(--space-2); }
.db-dot {
  width: 6px; height: 6px;
  border-radius: 50%;
  background: var(--ui-fg-4);
  display: inline-block;
}
.db-dot.ok   { background: var(--ui-success); }
.db-dot.fail { background: var(--ui-danger); }

.db-desc {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--space-2) var(--space-6);
}
@media (max-width: 720px) { .db-desc { grid-template-columns: 1fr; } }
.db-desc__cell {
  display: flex; align-items: center; gap: var(--space-3);
  padding: var(--space-2) 0;
  min-width: 0;
}
.db-desc__cell .lbl {
  flex-shrink: 0; width: 80px;
  font-size: var(--fs-xs); color: var(--ui-fg-3);
}
.db-desc__cell .val {
  font-size: var(--fs-sm); color: var(--ui-fg);
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis; min-width: 0;
}

.filter-inp { width: 180px; }
.modal-foot { display: flex; justify-content: flex-end; gap: var(--space-2); }

:deep(.mono) {
  font-family: var(--font-mono);
  font-size: var(--fs-xs);
  background: var(--ui-bg-2);
  border: 1px solid var(--ui-border);
  padding: 1px 6px;
  border-radius: var(--radius-sm);
  color: var(--ui-fg-2);
}
:deep(.text-danger) { color: var(--ui-danger-fg); }

.db-empty {
  display: flex; flex-direction: column; align-items: center;
  padding: var(--space-10) var(--space-6);
  text-align: center;
  background: var(--ui-bg-1);
  border: 1px solid var(--ui-border);
  border-radius: var(--radius-md);
}
.db-empty__icon { font-size: 42px; margin-bottom: var(--space-3); opacity: 0.8; }
.db-empty__title { font-size: var(--fs-md); font-weight: var(--fw-semibold); color: var(--ui-fg); }
.db-empty__sub { font-size: var(--fs-sm); color: var(--ui-fg-3); margin-top: var(--space-1); }
.db-empty__grid {
  margin-top: var(--space-6);
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: var(--space-3);
  width: 100%; max-width: 640px;
}
.db-empty__card-title { font-size: var(--fs-sm); font-weight: var(--fw-semibold); color: var(--ui-fg); margin-bottom: var(--space-3); text-align: left; }
.db-empty__card-body { display: flex; flex-direction: column; gap: var(--space-2); text-align: left; }
.db-empty__hint { font-size: var(--fs-xs); color: var(--ui-fg-3); line-height: var(--lh-relaxed); }
</style>
