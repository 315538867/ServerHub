<template>
  <div class="overview-page">
    <div class="info-row">
      <el-descriptions :column="3" border>
        <el-descriptions-item label="主机地址">{{ server?.host }}:{{ server?.port }}</el-descriptions-item>
        <el-descriptions-item label="登录用户">{{ server?.username }}</el-descriptions-item>
        <el-descriptions-item label="认证方式">{{ server?.auth_type === 'key' ? 'SSH 密钥' : '密码' }}</el-descriptions-item>
        <el-descriptions-item label="状态">
          <el-tag :type="statusTag(server?.status)" size="small">{{ server?.status ?? '—' }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="最后检测">{{ server?.last_check_at ?? '—' }}</el-descriptions-item>
        <el-descriptions-item label="备注">{{ server?.remark || '—' }}</el-descriptions-item>
      </el-descriptions>
      <div class="action-row">
        <el-button :loading="testing" @click="doTest">连接测试</el-button>
        <el-button :icon="Refresh" :loading="collecting" @click="doCollect">采集指标</el-button>
      </div>
    </div>

    <el-row :gutter="16" class="gauge-row">
      <el-col :span="8">
        <el-card shadow="never">
          <div class="gauge-label">CPU</div>
          <el-progress :percentage="latestMetric?.cpu ?? 0" :color="progressColor(latestMetric?.cpu ?? 0)" :stroke-width="14" />
          <div class="gauge-val">{{ (latestMetric?.cpu ?? 0).toFixed(1) }}%</div>
        </el-card>
      </el-col>
      <el-col :span="8">
        <el-card shadow="never">
          <div class="gauge-label">内存</div>
          <el-progress :percentage="latestMetric?.mem ?? 0" :color="progressColor(latestMetric?.mem ?? 0)" :stroke-width="14" />
          <div class="gauge-val">{{ (latestMetric?.mem ?? 0).toFixed(1) }}%</div>
        </el-card>
      </el-col>
      <el-col :span="8">
        <el-card shadow="never">
          <div class="gauge-label">磁盘</div>
          <el-progress :percentage="latestMetric?.disk ?? 0" :color="progressColor(latestMetric?.disk ?? 0)" :stroke-width="14" />
          <div class="gauge-val">{{ (latestMetric?.disk ?? 0).toFixed(1) }}%</div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="16" class="stat-row">
      <el-col :span="12">
        <el-card shadow="never" header="负载 (1min)">
          <span class="stat-val">{{ latestMetric?.load1?.toFixed(2) ?? '—' }}</span>
        </el-card>
      </el-col>
      <el-col :span="12">
        <el-card shadow="never" header="运行时间">
          <span class="stat-val">{{ formatUptime(latestMetric?.uptime) }}</span>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { Refresh } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { useServerStore } from '@/stores/server'
import { getServer, testServer, collectMetrics, getMetrics } from '@/api/servers'
import type { Server, Metric } from '@/types/api'

const route = useRoute()
const serverStore = useServerStore()
const serverId = computed(() => Number(route.params.serverId))
const server = ref<Server | null>(null)
const metrics = ref<Metric[]>([])
const latestMetric = computed(() => metrics.value[0] ?? null)
const testing = ref(false)
const collecting = ref(false)

function statusTag(status?: string) {
  return ({ online: 'success', offline: 'danger', unknown: 'info' } as Record<string, string>)[status ?? ''] ?? 'info'
}

function progressColor(val: number) {
  if (val >= 90) return '#f56c6c'
  if (val >= 70) return '#e6a23c'
  return '#67c23a'
}

function formatUptime(seconds?: number) {
  if (!seconds) return '—'
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  return d > 0 ? `${d}天 ${h}小时` : `${h}小时 ${m}分`
}

async function doTest() {
  testing.value = true
  try {
    const res = await testServer(serverId.value)
    if (res.status === 'ok') {
      ElMessage.success('连接成功')
      await serverStore.fetch()
    } else {
      ElMessage.error(`连接失败：${res.error ?? '未知错误'}`)
    }
  } catch { ElMessage.error('测试失败') }
  finally { testing.value = false }
}

async function doCollect() {
  collecting.value = true
  try {
    await collectMetrics(serverId.value)
    metrics.value = await getMetrics(serverId.value, 1)
    ElMessage.success('指标已更新')
  } catch { ElMessage.error('采集失败') }
  finally { collecting.value = false }
}

onMounted(async () => {
  server.value = await getServer(serverId.value)
  metrics.value = await getMetrics(serverId.value, 1)
})
</script>

<style scoped>
.overview-page { padding: 20px; }
.info-row { margin-bottom: 20px; }
.action-row { margin-top: 12px; display: flex; gap: 8px; }
.gauge-row { margin-bottom: 16px; }
.stat-row { margin-bottom: 16px; }
.gauge-label { font-size: 13px; color: #606266; margin-bottom: 8px; }
.gauge-val { text-align: right; font-size: 13px; color: #303133; margin-top: 6px; }
.stat-val { font-size: 28px; font-weight: 600; color: #303133; }
</style>
