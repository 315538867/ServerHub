<template>
  <div class="app-list">
    <div class="page-header">
      <h2>应用列表</h2>
      <el-button type="primary" @click="$router.push('/apps/create')">新建应用</el-button>
    </div>
    <el-table :data="appStore.apps" v-loading="appStore.loading" style="width: 100%">
      <el-table-column prop="name" label="应用名称">
        <template #default="{ row }">
          <el-link type="primary" @click="$router.push(`/apps/${row.id}/overview`)">{{ row.name }}</el-link>
        </template>
      </el-table-column>
      <el-table-column prop="domain" label="域名" />
      <el-table-column prop="status" label="状态">
        <template #default="{ row }">
          <el-tag :type="row.status === 'online' ? 'success' : row.status === 'offline' ? 'danger' : 'info'" size="small">{{ row.status }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="updated_at" label="更新时间" />
    </el-table>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useAppStore } from '@/stores/app'

const appStore = useAppStore()
onMounted(() => appStore.fetch())
</script>

<style scoped>
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
</style>
