<script setup lang="ts">
import { computed, ref } from 'vue'
import {
  NAlert, NButton, NCard, NInput, NRadioButton, NRadioGroup,
  NSpace, NTag, useMessage,
} from 'naive-ui'
import type { Deploy } from '@/types/api'

const props = defineProps<{ svc: Deploy | null }>()
const msg = useMessage()

// 三种日志源：按 Service.type 给合理默认，但用户可切换。
type Source = 'docker' | 'journald' | 'path'
const source = ref<Source>(guessInitial())
const tail = ref(200)
const customPath = ref('')
const unit = ref('')

function guessInitial(): Source {
  if (!props.svc) return 'path'
  if (props.svc.type === 'docker' || props.svc.type === 'docker-compose') return 'docker'
  if (props.svc.type === 'native') return 'journald'
  return 'path'
}

const dockerName = computed(() => props.svc ? `serverhub-svc-${props.svc.id}` : '')
const defaultUnit = computed(() => props.svc ? `serverhub-svc-${props.svc.id}.service` : '')

const cmd = computed(() => {
  const n = Math.max(1, Math.min(10000, tail.value || 200))
  switch (source.value) {
    case 'docker':
      return `docker logs --tail=${n} -f ${dockerName.value}`
    case 'journald': {
      const u = unit.value.trim() || defaultUnit.value
      return `journalctl -u ${shellQuote(u)} -n ${n} -f`
    }
    case 'path': {
      const p = customPath.value.trim()
      if (!p) return '# 请填入日志文件路径'
      return `tail -n ${n} -F ${shellQuote(p)}`
    }
  }
})

function shellQuote(s: string): string {
  if (/^[a-zA-Z0-9_./:@-]+$/.test(s)) return s
  return `'${s.replace(/'/g, `'\\''`)}'`
}

async function copy() {
  try {
    await navigator.clipboard.writeText(cmd.value)
    msg.success('已复制到剪贴板')
  } catch { msg.error('浏览器拒绝复制，请手动选中') }
}

function openTerminal() {
  if (!props.svc) return
  // 复用 Server Terminal。用户粘贴 cmd 执行。
  const url = `/panel/servers/${props.svc.server_id}/terminal?prefill=${encodeURIComponent(cmd.value)}`
  window.open(url, '_blank')
}
</script>

<template>
  <div>
    <NAlert type="info" :bordered="false" style="margin-bottom:12px">
      Service 新链路暂不直接代理日志流。下方根据 Service 类型给出常见日志命令；点"打开终端"跳到服务器终端并预填命令（M3 会把 WS 日志推送合并进来）。
    </NAlert>

    <NCard size="small" :bordered="false" style="margin-bottom:12px">
      <NSpace vertical :size="10">
        <NSpace>
          <NRadioGroup v-model:value="source" size="small">
            <NRadioButton value="docker">docker logs</NRadioButton>
            <NRadioButton value="journald">journalctl</NRadioButton>
            <NRadioButton value="path">自定义路径</NRadioButton>
          </NRadioGroup>
          <NInput
            v-model:value.number="tail" style="width:120px" size="small"
            placeholder="tail 行数"
          />
        </NSpace>

        <div v-if="source === 'docker'">
          <NTag size="small">容器名</NTag>
          <span style="margin-left:8px;font-family:monospace">{{ dockerName }}</span>
          <div style="color:#94a3b8;font-size:12px;margin-top:4px">
            约定由 Release Apply 使用的容器名，如已手工改名需自行调整命令。
          </div>
        </div>
        <div v-if="source === 'journald'">
          <NInput
            v-model:value="unit" size="small"
            :placeholder="`systemd unit，默认 ${defaultUnit}`"
          />
        </div>
        <div v-if="source === 'path'">
          <NInput
            v-model:value="customPath" size="small"
            :placeholder="svc?.work_dir ? `${svc.work_dir}/app.log` : '/var/log/xxx.log'"
          />
        </div>
      </NSpace>
    </NCard>

    <NCard size="small" :bordered="false" title="命令">
      <pre style="background:#0b1020;color:#e2e8f0;padding:12px;border-radius:4px;font-size:12px;white-space:pre-wrap">{{ cmd }}</pre>
      <NSpace justify="end" style="margin-top:8px">
        <NButton size="small" @click="copy">复制</NButton>
        <NButton size="small" type="primary" :disabled="!svc" @click="openTerminal">
          打开终端并预填
        </NButton>
      </NSpace>
    </NCard>
  </div>
</template>
