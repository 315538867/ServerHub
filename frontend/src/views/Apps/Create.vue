<template>
  <div class="page-container">
    <div class="section-block" style="max-width: 720px;">
      <div class="section-title">
        <span class="title-text">新建应用</span>
      </div>
      <div class="form-wrap">
        <div class="form-section-label">基本信息</div>
        <t-form :data="form" label-width="90px" colon>
          <t-form-item label="应用名称" name="name">
            <t-input v-model="form.name" placeholder="例如：my-blog" />
          </t-form-item>
          <t-form-item label="描述">
            <t-textarea v-model="form.description" :autosize="{ minRows: 2 }" placeholder="应用描述（可选）" />
          </t-form-item>
          <t-form-item label="域名">
            <t-input v-model="form.domain" placeholder="blog.example.com" />
          </t-form-item>
        </t-form>

        <t-divider />
        <div class="form-section-label">服务配置</div>
        <t-form :data="form" label-width="90px" colon>
          <t-form-item label="服务器" name="server_id">
            <t-select v-model="form.server_id" placeholder="选择服务器" style="width:100%">
              <t-option v-for="s in serverStore.servers" :key="s.id" :label="s.name" :value="s.id" />
            </t-select>
          </t-form-item>
          <t-form-item label="容器名">
            <t-input v-model="form.container_name" placeholder="Docker 容器名" />
          </t-form-item>
        </t-form>

        <t-divider />
        <div class="form-section-label">部署配置</div>
        <t-form :data="form" label-width="90px" colon>
          <t-form-item label="Nginx 站点">
            <t-input v-model="form.site_name" placeholder="conf.d 中的文件名" />
          </t-form-item>
        </t-form>

        <div class="form-footer">
          <t-button theme="primary" :loading="saving" @click="handleCreate">创建应用</t-button>
          <t-button variant="outline" @click="$router.back()">取消</t-button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import { createApp } from '@/api/application'
import { useServerStore } from '@/stores/server'
import { useAppStore } from '@/stores/app'

const router = useRouter()
const serverStore = useServerStore()
const appStore = useAppStore()
const saving = ref(false)

const form = reactive({
  name: '', description: '',
  server_id: null as number | null,
  domain: '', site_name: '', container_name: '',
  deploy_id: null as number | null, db_conn_id: null as number | null,
})

onMounted(() => serverStore.fetch())

async function handleCreate() {
  if (!form.name || !form.server_id) { MessagePlugin.warning('请填写必填项'); return }
  saving.value = true
  try {
    const app = await createApp(form as any)
    MessagePlugin.success('应用创建成功')
    await appStore.fetch()
    router.push(`/apps/${app.id}/overview`)
  } catch (e: any) {
    MessagePlugin.error(e.message || '创建失败')
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.title-text {
  font-size: 14px;
  font-weight: 600;
  color: var(--sh-text-primary);
}
.form-wrap {
  padding: 20px 28px 28px;
}
.form-section-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--sh-text-primary);
  margin-bottom: 16px;
}
.form-footer {
  display: flex;
  gap: 12px;
  margin-top: 28px;
  padding-top: 20px;
  border-top: 1px solid var(--sh-border);
}
:deep(.t-divider) {
  margin: 20px 0;
}
</style>
