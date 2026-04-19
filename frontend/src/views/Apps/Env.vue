<template>
  <div class="env-page">
    <template v-if="app?.deploy_id">
      <div class="page-toolbar">
        <el-button type="primary" size="small" @click="addRow">添加变量</el-button>
        <el-button size="small" :loading="saving" @click="saveEnv">保存</el-button>
        <el-button size="small" @click="loadEnv">刷新</el-button>
      </div>

      <el-table :data="envVars" v-loading="loading" style="width:100%" empty-text="暂无环境变量">
        <el-table-column label="键" min-width="200">
          <template #default="{ row }">
            <el-input v-model="row.key" placeholder="ENV_KEY" size="small" />
          </template>
        </el-table-column>
        <el-table-column label="值" min-width="300">
          <template #default="{ row }">
            <el-input
              v-model="row.value"
              :type="row.secret && !row.revealed ? 'password'"
              :placeholder="row.secret ? '••••••••' : 'value'"
              size="small"
              show-password
            />
          </template>
        </el-table-column>
        <el-table-column label="加密" width="80" align="center">
          <template #default="{ row }">
            <el-checkbox v-model="row.secret" />
          </template>
        </el-table-column>
        <el-table-column label="操作" width="80" fixed="right">
          <template #default="{ $index }">
            <el-button size="small" type="danger" @click="removeRow($index)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="webhook-block">
        <el-divider>Webhook 触发部署</el-divider>
        <div v-if="webhook">
          <el-form label-width="80px">
            <el-form-item label="Webhook URL">
              <el-input :value="webhook.url" readonly>
                <template #append>
                  <el-button @click="copyWebhook(webhook!.url)">复制</el-button>
                </template>
              </el-input>
            </el-form-item>
            <el-form-item label="Secret">
              <el-input :value="webhook.secret" type="password" show-password readonly />
            </el-form-item>
          </el-form>
        </div>
        <el-button v-else size="small" @click="loadWebhook">查看 Webhook 配置</el-button>
      </div>
    </template>
    <el-empty v-else description="该应用未关联部署配置，无法管理环境变量" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
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

async function loadEnv() {
  if (!app.value?.deploy_id) return
  loading.value = true
  try {
    const vars = await getDeployEnv(app.value.deploy_id)
    envVars.value = vars.map(v => ({ ...v, revealed: false }))
  } catch { ElMessage.error('加载失败') }
  finally { loading.value = false }
}

async function saveEnv() {
  if (!app.value?.deploy_id) return
  saving.value = true
  try {
    await putDeployEnv(app.value.deploy_id, envVars.value.map(({ key, value, secret }) => ({ key, value, secret })))
    ElMessage.success('已保存')
    await loadEnv()
  } catch { ElMessage.error('保存失败') }
  finally { saving.value = false }
}

function addRow() { envVars.value.push({ key: '', value: '', secret: false, revealed: false }) }
function removeRow(idx: number) { envVars.value.splice(idx, 1) }

async function loadWebhook() {
  if (!app.value?.deploy_id) return
  try { webhook.value = await getWebhookInfo(app.value.deploy_id) }
  catch { ElMessage.error('加载失败') }
}

function copyWebhook(url: string) {
  navigator.clipboard.writeText(url)
  ElMessage.success('已复制到剪贴板')
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
</style>
