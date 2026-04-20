<template>
  <div class="page-container">
    <template v-if="app?.deploy_id">
      <!-- 环境变量 -->
      <div class="section-block">
        <div class="section-title">
          <span class="title-text">环境变量</span>
          <t-space size="small">
            <t-button size="small" theme="primary" @click="addRow">添加变量</t-button>
            <t-button size="small" :loading="saving" @click="saveEnv">保存</t-button>
            <t-button size="small" variant="outline" @click="loadEnv">刷新</t-button>
          </t-space>
        </div>
        <div class="table-wrap">
          <t-table :data="envVars" :columns="envColumns" :loading="loading" row-key="key" empty="暂无环境变量" stripe size="small">
            <template #key="{ row }">
              <t-input v-model="row.key" placeholder="ENV_KEY" size="small" />
            </template>
            <template #value="{ row }">
              <t-input
                v-model="row.value"
                :type="row.secret && !row.revealed ? 'password' : 'text'"
                :placeholder="row.secret ? '••••••••' : 'value'"
                size="small"
              />
            </template>
            <template #secret="{ row }">
              <t-checkbox v-model="row.secret" />
            </template>
            <template #operations="{ rowIndex }">
              <t-button theme="danger" size="small" variant="text" @click="removeRow(rowIndex)">删除</t-button>
            </template>
          </t-table>
        </div>
      </div>

      <!-- Webhook -->
      <div class="section-block">
        <div class="section-title">
          <span class="title-text">Webhook 触发部署</span>
        </div>
        <div class="webhook-wrap">
          <div v-if="webhook">
            <t-form label-width="100px" colon>
              <t-form-item label="Webhook URL">
                <div class="input-with-btn">
                  <t-input :value="webhook.url" readonly />
                  <t-button size="small" @click="copyWebhook(webhook!.url)">复制</t-button>
                </div>
              </t-form-item>
              <t-form-item label="Secret">
                <t-input :value="webhook.secret" type="password" readonly />
              </t-form-item>
            </t-form>
          </div>
          <t-button v-else size="small" variant="outline" @click="loadWebhook">查看 Webhook 配置</t-button>
        </div>
      </div>
    </template>
    <div v-else class="section-block empty-block">
      <t-empty description="该应用未关联部署配置，无法管理环境变量" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import { useAppStore } from '@/stores/app'
import { getDeployEnv, putDeployEnv, getWebhookInfo } from '@/api/deploy'
import type { EnvVar } from '@/api/deploy'

const route = useRoute()
const appStore = useAppStore()
const appId = computed(() => Number(route.params.appId))
const app = computed(() => appStore.getById(appId.value))

interface EnvRow extends EnvVar { revealed: boolean }

const envVars = ref<EnvRow[]>([])
const loading = ref(false)
const saving = ref(false)
const webhook = ref<{ url: string; secret: string } | null>(null)

const envColumns = [
  { colKey: 'key', title: '键', minWidth: 200 },
  { colKey: 'value', title: '值', minWidth: 300 },
  { colKey: 'secret', title: '加密', width: 80, align: 'center' as const },
  { colKey: 'operations', title: '操作', width: 80, fixed: 'right' as const },
]

async function loadEnv() {
  if (!app.value?.deploy_id) return
  loading.value = true
  try {
    const vars = await getDeployEnv(app.value.deploy_id)
    envVars.value = vars.map(v => ({ ...v, revealed: false }))
  } catch { MessagePlugin.error('加载失败') }
  finally { loading.value = false }
}

async function saveEnv() {
  if (!app.value?.deploy_id) return
  saving.value = true
  try {
    await putDeployEnv(app.value.deploy_id, envVars.value.map(({ key, value, secret }) => ({ key, value, secret })))
    MessagePlugin.success('已保存')
    await loadEnv()
  } catch { MessagePlugin.error('保存失败') }
  finally { saving.value = false }
}

function addRow() { envVars.value.push({ key: '', value: '', secret: false, revealed: false }) }
function removeRow(idx: number) { envVars.value.splice(idx, 1) }

async function loadWebhook() {
  if (!app.value?.deploy_id) return
  try { webhook.value = await getWebhookInfo(app.value.deploy_id) }
  catch { MessagePlugin.error('加载失败') }
}

function copyWebhook(url: string) {
  navigator.clipboard.writeText(url)
  MessagePlugin.success('已复制到剪贴板')
}

onMounted(async () => {
  if (!appStore.apps.length) await appStore.fetch()
  await loadEnv()
})
</script>

<style scoped>
.title-text {
  font-size: 14px;
  font-weight: 600;
  color: var(--sh-text-primary);
}
.table-wrap {
  padding: 0 20px 16px;
}
:deep(.t-table th) {
  background: #FAFAFA;
  font-size: 12px;
  color: var(--sh-text-secondary);
  font-weight: 500;
}
:deep(.t-table td) {
  font-size: 13px;
}
.webhook-wrap {
  padding: 16px 20px 20px;
}
.empty-block {
  padding: 40px 20px;
  display: flex;
  justify-content: center;
}
.input-with-btn {
  display: flex;
  gap: 8px;
  align-items: center;
  width: 100%;
}
.input-with-btn .t-input { flex: 1; }
</style>
