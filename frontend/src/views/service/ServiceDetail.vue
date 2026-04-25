<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { NCard, NTabs, NTabPane, NSpin, NTag, NText } from 'naive-ui'
import { useRoute } from 'vue-router'
import ReleasesTab from './tabs/ReleasesTab.vue'
import EnvTab from './tabs/EnvTab.vue'
import ConfigTab from './tabs/ConfigTab.vue'
import DeploysTab from './tabs/DeploysTab.vue'
import SettingsTab from './tabs/SettingsTab.vue'
import ArtifactsTab from './tabs/ArtifactsTab.vue'
import LogsTab from './tabs/LogsTab.vue'
import { getDeploy } from '@/api/deploy'
import type { Deploy } from '@/types/api'

const route = useRoute()
const sid = computed(() => Number(route.params.id))
const svc = ref<Deploy | null>(null)
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    svc.value = await getDeploy(sid.value)
  } finally {
    loading.value = false
  }
}
onMounted(load)
</script>

<template>
  <div class="service-detail">
    <NSpin :show="loading">
      <NCard size="small" :bordered="false">
        <template #header>
          <span style="font-weight:600">{{ svc?.name || `Service #${sid}` }}</span>
          <NTag v-if="svc?.type" size="small" style="margin-left:8px">{{ svc.type }}</NTag>
        </template>
        <NText depth="3" style="font-size:12px">
          server_id={{ svc?.server_id }} · current_release_id={{ svc?.current_release_id ?? '—' }}
        </NText>
      </NCard>

      <NTabs default-value="releases" animated style="margin-top:12px">
        <NTabPane name="overview" tab="概览">
          <NCard size="small">服务概览（M2 完善）</NCard>
        </NTabPane>
        <NTabPane name="releases" tab="Release">
          <ReleasesTab :sid="sid" />
        </NTabPane>
        <NTabPane name="env" tab="环境变量">
          <EnvTab :sid="sid" />
        </NTabPane>
        <NTabPane name="config" tab="配置文件">
          <ConfigTab :sid="sid" />
        </NTabPane>
        <NTabPane name="deploys" tab="部署历史">
          <DeploysTab :sid="sid" />
        </NTabPane>
        <NTabPane name="artifacts" tab="制品">
          <ArtifactsTab :sid="sid" />
        </NTabPane>
        <NTabPane name="logs" tab="日志">
          <LogsTab :svc="svc" />
        </NTabPane>
        <NTabPane name="settings" tab="设置">
          <SettingsTab :sid="sid" :auto-rollback="!!svc?.auto_rollback_on_fail" @refresh="load" />
        </NTabPane>
      </NTabs>
    </NSpin>
  </div>
</template>

<style scoped>
.service-detail { padding: 16px; }
</style>
