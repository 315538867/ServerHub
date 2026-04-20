<template>
  <div class="env-page">
    <template v-if="app?.deploy_id">
      <div class="page-toolbar">
        <t-button theme="primary" size="small" @click="addRow">添加变量</t-button>
        <t-button size="small" :loading="saving" @click="saveEnv">保存</t-button>
        <t-button size="small" variant="outline" @click="loadEnv">刷新</t-button>
      </div>

      <t-table :data="envVars" :columns="envColumns" :loading="loading" row-key="key" empty="暂无环境变量" stripe>
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

      <div class="webhook-block">
        <div class="section-divider"><span>Webhook 触发部署</span></div>
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
        <t-button v-else size="small" @click="loadWebhook">查看 Webhook 配置</t-button>
      </div>
    </template>
    <t-empty v-else description="该应用未关联部署配置，无法管理环境变量" />
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
.env-page { padding: 4px 0; }
.page-toolbar { display: flex; gap: 8px; align-items: center; margin-bottom: 16px; }
.webhook-block { margin-top: 8px; }
.section-divider {
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 20px 0 16px;
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
.input-with-btn { display: flex; gap: 8px; align-items: center; width: 100%; }
.input-with-btn .t-input { flex: 1; }
</style>
