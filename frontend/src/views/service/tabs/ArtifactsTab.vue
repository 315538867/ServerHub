<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue'
import {
  NButton, NCard, NDataTable, NForm, NFormItem, NInput, NInputNumber,
  NModal, NRadioButton, NRadioGroup, NSpace, NTag, NUpload, NUploadDragger,
  useMessage,
} from 'naive-ui'
import type { DataTableColumns, UploadFileInfo } from 'naive-ui'
import { createArtifact, listArtifacts, uploadArtifact } from '@/api/release'
import type { Artifact, ArtifactProvider, CreateArtifactPayload } from '@/types/release'

const props = defineProps<{ sid: number }>()
const msg = useMessage()
const rows = ref<Artifact[]>([])
const loading = ref(false)
const show = ref(false)

// 表单：先选 provider 再展示对应字段
type FormProvider = Exclude<ArtifactProvider, 'imported'>
const providerOptions: { label: string; value: FormProvider; hint: string }[] = [
  { label: 'Docker 镜像', value: 'docker', hint: '如 nginx:alpine 或 registry.example.com/app:v1.2' },
  { label: 'Git 仓库', value: 'git', hint: '如 https://github.com/foo/bar.git 或 https://github.com/foo/bar.git#v1.0（# 后为分支/tag/sha）' },
  { label: 'HTTP 文件', value: 'http', hint: '直链 URL，curl -fsSL 可拉到' },
  { label: '拉取脚本', value: 'script', hint: 'bash 片段，执行后产物须出现在当前工作目录' },
  { label: '本地上传', value: 'upload', hint: '选一个本地文件上传，面板计算 sha256 去重存档' },
]
const form = ref<{
  provider: FormProvider
  ref: string
  pull_script: string
  file: UploadFileInfo | null
}>({
  provider: 'docker', ref: '', pull_script: '', file: null,
})
const submitting = ref(false)

const currentHint = computed(
  () => providerOptions.find(o => o.value === form.value.provider)?.hint ?? '',
)

async function load() {
  loading.value = true
  try { rows.value = await listArtifacts(props.sid) }
  finally { loading.value = false }
}

function openDialog() {
  form.value = { provider: 'docker', ref: '', pull_script: '', file: null }
  show.value = true
}

async function submit() {
  submitting.value = true
  try {
    if (form.value.provider === 'upload') {
      const f = form.value.file?.file
      if (!f) { msg.error('请选择要上传的文件'); return }
      await uploadArtifact(props.sid, f)
    } else {
      const payload: CreateArtifactPayload = { provider: form.value.provider }
      if (form.value.provider === 'script') {
        if (!form.value.pull_script.trim()) {
          msg.error('脚本内容不能为空'); return
        }
        payload.pull_script = form.value.pull_script
      } else {
        if (!form.value.ref.trim()) {
          msg.error('ref 不能为空'); return
        }
        payload.ref = form.value.ref.trim()
      }
      await createArtifact(props.sid, payload)
    }
    msg.success('已创建制品')
    show.value = false
    await load()
  } catch (e: any) {
    msg.error(e?.message || '创建失败')
  } finally {
    submitting.value = false
  }
}

const providerTone: Record<ArtifactProvider, 'default' | 'success' | 'warning' | 'info' | 'error'> = {
  docker: 'info', git: 'success', http: 'default', script: 'warning', upload: 'default', imported: 'error',
}

const columns: DataTableColumns<Artifact> = [
  { title: 'ID', key: 'id', width: 60 },
  {
    title: 'Provider', key: 'provider', width: 100,
    render: r => h(NTag, { size: 'small', type: providerTone[r.provider] }, { default: () => r.provider }),
  },
  { title: 'Ref', key: 'ref', ellipsis: { tooltip: true } },
  { title: 'Checksum', key: 'checksum', width: 200, ellipsis: { tooltip: true } },
  {
    title: 'Size', key: 'size_bytes', width: 100,
    render: r => r.size_bytes ? humanSize(r.size_bytes) : '',
  },
  { title: 'Created', key: 'created_at', width: 180 },
]

function humanSize(n: number): string {
  const u = ['B', 'KB', 'MB', 'GB']
  let i = 0
  while (n >= 1024 && i < u.length - 1) { n /= 1024; i++ }
  return `${n.toFixed(i === 0 ? 0 : 1)} ${u[i]}`
}

function onFileChange(data: { fileList: UploadFileInfo[] }) {
  form.value.file = data.fileList[0] || null
}

onMounted(load)
</script>

<template>
  <div>
    <NSpace justify="space-between" style="margin-bottom:8px">
      <NButton size="small" type="primary" @click="openDialog">+ 新建制品</NButton>
      <NButton size="small" @click="load">刷新</NButton>
    </NSpace>
    <NDataTable :columns="columns" :data="rows" :loading="loading" size="small" />

    <NModal v-model:show="show" preset="card" title="新建 Artifact" style="width:640px">
      <NForm label-placement="left" label-width="90">
        <NFormItem label="Provider">
          <NRadioGroup v-model:value="form.provider" size="small">
            <NRadioButton v-for="o in providerOptions" :key="o.value" :value="o.value">
              {{ o.label }}
            </NRadioButton>
          </NRadioGroup>
        </NFormItem>
        <NFormItem v-if="form.provider !== 'upload' && form.provider !== 'script'" label="Ref">
          <NInput v-model:value="form.ref" :placeholder="currentHint" />
        </NFormItem>
        <NFormItem v-if="form.provider === 'script'" label="拉取脚本">
          <NInput
            v-model:value="form.pull_script"
            type="textarea" :autosize="{ minRows: 6, maxRows: 16 }"
            :placeholder="currentHint"
          />
        </NFormItem>
        <NFormItem v-if="form.provider === 'upload'" label="文件">
          <NUpload
            :max="1" :default-upload="false" @change="onFileChange"
          >
            <NUploadDragger>
              <div style="padding:20px 0;font-size:13px">点击或拖入要上传的文件（单文件，面板侧计算 sha256）</div>
            </NUploadDragger>
          </NUpload>
        </NFormItem>
        <div style="color:#94a3b8;font-size:12px;margin-bottom:8px">
          {{ currentHint }}
        </div>
        <NSpace justify="end">
          <NButton size="small" @click="show = false">取消</NButton>
          <NButton size="small" type="primary" :loading="submitting" @click="submit">创建</NButton>
        </NSpace>
      </NForm>
    </NModal>
  </div>
</template>
