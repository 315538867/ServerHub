<template>
  <div class="app-create">
    <h2>新建应用</h2>
    <el-form :model="form" label-width="100px" style="max-width: 600px">
      <el-form-item label="应用名称" required>
        <el-input v-model="form.name" placeholder="例如：my-blog" />
      </el-form-item>
      <el-form-item label="描述">
        <el-input v-model="form.description" type="textarea" :rows="2" />
      </el-form-item>
      <el-form-item label="服务器" required>
        <el-select v-model="form.server_id" placeholder="选择服务器" style="width: 100%">
          <el-option v-for="s in serverStore.servers" :key="s.id" :label="s.name" :value="s.id" />
        </el-select>
      </el-form-item>
      <el-form-item label="域名">
        <el-input v-model="form.domain" placeholder="blog.example.com" />
      </el-form-item>
      <el-form-item label="Nginx 站点">
        <el-input v-model="form.site_name" placeholder="conf.d 中的文件名" />
      </el-form-item>
      <el-form-item label="容器名">
        <el-input v-model="form.container_name" placeholder="Docker 容器名" />
      </el-form-item>
      <el-form-item>
        <el-button type="primary" @click="handleCreate" :loading="saving">创建</el-button>
        <el-button @click="$router.back()">取消</el-button>
      </el-form-item>
    </el-form>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { createApp } from '@/api/application'
import { useServerStore } from '@/stores/server'
import { useAppStore } from '@/stores/app'

const router = useRouter()
const serverStore = useServerStore()
const appStore = useAppStore()
const saving = ref(false)

const form = reactive({
  name: '',
  description: '',
  server_id: null as number | null,
  domain: '',
  site_name: '',
  container_name: '',
  deploy_id: null as number | null,
  db_conn_id: null as number | null,
})

onMounted(() => serverStore.fetch())

async function handleCreate() {
  if (!form.name || !form.server_id) {
    ElMessage.warning('请填写必填项')
    return
  }
  saving.value = true
  try {
    const app = await createApp(form as any)
    ElMessage.success('应用创建成功')
    await appStore.fetch()
    router.push(`/apps/${app.id}/overview`)
  } catch (e: any) {
    ElMessage.error(e.message || '创建失败')
  } finally {
    saving.value = false
  }
}
</script>
