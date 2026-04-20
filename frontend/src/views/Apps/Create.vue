<template>
  <div class="page-container create-page">
    <div class="create-card">

      <!-- 页面标题 -->
      <div class="create-header">
        <h2 class="create-title">新建应用</h2>
        <p class="create-subtitle">配置应用的基础信息、关联服务与部署资源</p>
      </div>

      <!-- 表单内容 -->
      <div class="create-body">

        <!-- 基本信息 -->
        <div class="form-section">
          <div class="form-section-title">基本信息</div>
          <div class="form-grid">
            <div class="form-field">
              <label class="form-label">应用名称 <span class="form-required">*</span></label>
              <t-input v-model="form.name" placeholder="例如：my-blog" />
              <span class="form-hint">唯一标识符，创建后不可修改</span>
            </div>
            <div class="form-field">
              <label class="form-label">描述</label>
              <t-input v-model="form.description" placeholder="简短描述该应用的用途（可选）" />
            </div>
            <div class="form-field">
              <label class="form-label">域名</label>
              <t-input v-model="form.domain" placeholder="blog.example.com（可选）" />
            </div>
          </div>
        </div>

        <!-- 服务配置 -->
        <div class="form-section">
          <div class="form-section-title">服务配置</div>
          <div class="form-grid">
            <div class="form-field">
              <label class="form-label">关联服务器 <span class="form-required">*</span></label>
              <t-select v-model="form.server_id" placeholder="选择服务器">
                <t-option v-for="s in serverStore.servers" :key="s.id" :label="s.name" :value="s.id" />
              </t-select>
            </div>
            <div class="form-field">
              <label class="form-label">Docker 容器名</label>
              <t-input v-model="form.container_name" placeholder="关联的容器名（可选）" />
              <span class="form-hint">填写后开启「服务」Tab，用于管理 Docker 容器</span>
            </div>
          </div>
        </div>

        <!-- 部署配置 -->
        <div class="form-section form-section--last">
          <div class="form-section-title">部署配置</div>
          <div class="form-grid">
            <div class="form-field">
              <label class="form-label">Nginx 站点</label>
              <t-input v-model="form.site_name" placeholder="conf.d 中的配置文件名（可选）" />
              <span class="form-hint">填写后开启「域名」Tab，用于管理对应的 Nginx 站点配置</span>
            </div>
            <div class="form-field">
              <label class="form-label">应用基础目录</label>
              <t-input v-model="form.base_dir" placeholder="/srv/apps/my-blog" />
              <span class="form-hint">在服务器上自动创建 data / logs / config / backup，留空按应用名自动填充</span>
            </div>
            <div class="form-field">
              <label class="form-label">Nginx 暴露方式</label>
              <t-radio-group v-model="form.expose_mode" variant="default-filled">
                <t-radio-button value="none">不暴露</t-radio-button>
                <t-radio-button value="path">路径转发</t-radio-button>
                <t-radio-button value="site">独立站点</t-radio-button>
              </t-radio-group>
              <span v-if="form.expose_mode === 'none'" class="form-hint">不通过 Nginx 暴露，「路由配置」Tab 将隐藏</span>
              <span v-else-if="form.expose_mode === 'path'" class="form-hint">将应用路径反代到已有 Nginx，创建后在「路由配置」Tab 配置路由</span>
              <span v-else class="form-hint">为应用创建独立 Nginx 站点，需填写域名；创建后在「路由配置」Tab 配置路由</span>
            </div>
          </div>
        </div>

      </div>

      <!-- 底部操作 -->
      <div class="create-footer">
        <t-button theme="primary" :loading="saving" @click="handleCreate">创建应用</t-button>
        <t-button variant="outline" @click="$router.back()">取消</t-button>
      </div>

    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, watch, onMounted } from 'vue'
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
  base_dir: '',
  expose_mode: 'none' as 'none' | 'path' | 'site',
  deploy_id: null as number | null, db_conn_id: null as number | null,
})

watch(() => form.name, (name, oldName) => {
  const autoOld = oldName ? `/srv/apps/${oldName}` : ''
  if (!form.base_dir || form.base_dir === autoOld) {
    form.base_dir = name ? `/srv/apps/${name}` : ''
  }
})

onMounted(() => serverStore.fetch())

async function handleCreate() {
  if (!form.name || !form.server_id) { MessagePlugin.warning('请填写应用名称和服务器'); return }
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
.create-page {
  display: flex;
  justify-content: center;
  align-items: flex-start;
}

.create-card {
  width: 100%;
  max-width: 680px;
  background: var(--sh-card-bg);
  border: var(--sh-card-border);
  border-radius: var(--sh-card-radius);
  box-shadow: var(--sh-card-shadow);
  overflow: hidden;
}

/* 页面标题 */
.create-header {
  padding: 24px 32px 22px;
  border-bottom: 1px solid var(--sh-border);
}
.create-title {
  margin: 0 0 5px;
  font-size: 16px;
  font-weight: 600;
  color: var(--sh-text-primary);
}
.create-subtitle {
  margin: 0;
  font-size: 13px;
  color: var(--sh-text-secondary);
}

/* 表单主体 */
.create-body {
  padding: 4px 0;
}

/* 分区块 */
.form-section {
  padding: 22px 32px 20px;
  border-bottom: 1px solid var(--sh-border);
}
.form-section--last {
  border-bottom: none;
}

.form-section-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--sh-text-primary);
  border-left: 3px solid var(--sh-blue);
  padding-left: 10px;
  margin-bottom: 18px;
  line-height: 1.4;
}

/* 表单网格 */
.form-grid {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.form-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-label {
  font-size: 13px;
  color: var(--sh-text-primary);
  font-weight: 500;
  line-height: 1;
}

.form-required {
  color: #e34d59;
  margin-left: 2px;
}

.form-hint {
  font-size: 12px;
  color: var(--sh-text-secondary);
  line-height: 1.4;
}

/* 底部操作 */
.create-footer {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 18px 32px;
  border-top: 1px solid var(--sh-border);
  background: var(--sh-page-bg);
}
</style>
