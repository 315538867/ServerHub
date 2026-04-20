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
              <span class="form-hint">关联服务器上已存在的容器（如 my-nginx），填写后开启「服务」Tab，可查看容器状态、重启/停止并实时查看日志</span>
            </div>
          </div>
        </div>

        <!-- 部署配置 -->
        <div class="form-section">
          <div class="form-section-title">Nginx 配置</div>
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
              <span v-else class="form-hint">为应用创建独立 Nginx 站点，创建后在「路由配置」Tab 配置路由</span>
            </div>
            <div v-if="form.expose_mode === 'site'" class="form-field">
              <label class="form-label">域名 <span class="form-required">*</span></label>
              <t-input v-model="form.domain" placeholder="blog.example.com" />
              <span class="form-hint">独立站点的访问域名，Nginx 将以此域名生成 server_name 配置</span>
            </div>
          </div>
        </div>

        <!-- 部署方式 -->
        <div class="form-section form-section--last">
          <div class="form-section-title">部署方式 <span class="form-hint" style="font-weight:400;border:none;padding:0">（可选，稍后在「部署」Tab 配置）</span></div>
          <div class="deploy-type-cards">
            <div
              v-for="t in deployTypes" :key="t.value"
              class="deploy-type-card"
              :class="{ 'is-active': deployType === t.value }"
              @click="deployType = deployType === t.value ? '' : t.value"
            >
              <div class="dtc-icon">{{ t.icon }}</div>
              <div class="dtc-title">{{ t.label }}</div>
              <div class="dtc-desc">{{ t.desc }}</div>
            </div>
          </div>

          <div v-if="deployType" class="deploy-type-fields">
            <div class="form-grid">
              <div class="form-field">
                <label class="form-label">工作目录</label>
                <t-input v-model="deployForm.work_dir" placeholder="例如：/srv/apps/my-blog" />
              </div>
              <template v-if="deployType === 'docker-compose'">
                <div class="form-field">
                  <label class="form-label">Compose 文件名</label>
                  <t-input v-model="deployForm.compose_file" placeholder="docker-compose.yml" />
                </div>
                <div class="form-field">
                  <label class="form-label">镜像名（可选）</label>
                  <t-input v-model="deployForm.image_name" placeholder="例如：nginx:latest" />
                </div>
              </template>
              <template v-if="deployType === 'docker'">
                <div class="form-field">
                  <label class="form-label">镜像名 <span class="form-required">*</span></label>
                  <t-input v-model="deployForm.image_name" placeholder="例如：nginx:latest" />
                </div>
                <div class="form-field">
                  <label class="form-label">启动命令（可选）</label>
                  <t-input v-model="deployForm.start_cmd" placeholder="docker run 附加参数" />
                </div>
              </template>
              <template v-if="deployType === 'native'">
                <div class="form-field">
                  <label class="form-label">启动命令 <span class="form-required">*</span></label>
                  <t-input v-model="deployForm.start_cmd" placeholder="例如：./server --port 8080" />
                </div>
              </template>
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
import { createApp, updateApp } from '@/api/application'
import { createDeploy } from '@/api/deploy'
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

const deployTypes = [
  { value: 'docker-compose', icon: '🐙', label: 'Docker Compose', desc: '通过 compose 文件管理多容器服务' },
  { value: 'docker',         icon: '🐳', label: 'Docker',         desc: '直接拉取镜像并运行单个容器' },
  { value: 'native',         icon: '📦', label: '文件部署',        desc: '上传可执行文件，配置启动命令' },
] as const
type DeployTypeVal = typeof deployTypes[number]['value']

const deployType = ref<DeployTypeVal | ''>('')
const deployForm = reactive({ work_dir: '', compose_file: 'docker-compose.yml', image_name: '', start_cmd: '' })

watch(() => form.name, (name, oldName) => {
  const autoOld = oldName ? `/srv/apps/${oldName}` : ''
  if (!form.base_dir || form.base_dir === autoOld) {
    form.base_dir = name ? `/srv/apps/${name}` : ''
  }
})

watch(() => form.base_dir, (dir) => {
  if (!deployForm.work_dir || deployForm.work_dir === '') {
    deployForm.work_dir = dir
  }
})

onMounted(() => serverStore.fetch())

async function handleCreate() {
  if (!form.name || !form.server_id) { MessagePlugin.warning('请填写应用名称和服务器'); return }
  if (form.expose_mode === 'site' && !form.domain) { MessagePlugin.warning('独立站点模式需填写域名'); return }
  if (deployType.value === 'docker' && !deployForm.image_name) { MessagePlugin.warning('Docker 部署需填写镜像名'); return }
  if (deployType.value === 'native' && !deployForm.start_cmd) { MessagePlugin.warning('文件部署需填写启动命令'); return }
  saving.value = true
  try {
    const app = await createApp(form as any)
    if (deployType.value) {
      const deploy = await createDeploy({
        name: app.name,
        server_id: app.server_id,
        type: deployType.value,
        work_dir: deployForm.work_dir,
        compose_file: deployForm.compose_file || 'docker-compose.yml',
        image_name: deployForm.image_name,
        start_cmd: deployForm.start_cmd,
      })
      await updateApp(app.id, { ...app, deploy_id: deploy.id } as any)
      MessagePlugin.success('应用创建成功，已关联部署配置')
      await appStore.fetch()
      router.push(`/apps/${app.id}/deploy`)
    } else {
      MessagePlugin.success('应用创建成功')
      await appStore.fetch()
      router.push(`/apps/${app.id}/overview`)
    }
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

/* 部署类型卡片 */
.deploy-type-cards {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
  margin-bottom: 16px;
}

.deploy-type-card {
  border: 1px solid var(--sh-border);
  border-radius: 8px;
  padding: 14px 12px;
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s;
  text-align: center;
}
.deploy-type-card:hover {
  border-color: var(--sh-blue);
  background: color-mix(in srgb, var(--sh-blue) 5%, transparent);
}
.deploy-type-card.is-active {
  border-color: var(--sh-blue);
  background: color-mix(in srgb, var(--sh-blue) 8%, transparent);
}
.dtc-icon { font-size: 22px; margin-bottom: 6px; }
.dtc-title { font-size: 13px; font-weight: 600; color: var(--sh-text-primary); margin-bottom: 4px; }
.dtc-desc  { font-size: 11px; color: var(--sh-text-secondary); line-height: 1.4; }

.deploy-type-fields {
  border-top: 1px dashed var(--sh-border);
  padding-top: 16px;
}
</style>
