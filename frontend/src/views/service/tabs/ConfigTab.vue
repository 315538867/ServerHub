<script setup lang="ts">
import { onMounted, ref } from 'vue'
import {
  NButton, NDataTable, NForm, NFormItem, NInput, NInputNumber,
  NModal, NSpace, NDynamicInput, useMessage,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { createConfigSet, listConfigSets } from '@/api/release'
import type { ConfigFileItem, ConfigFileSet } from '@/types/release'

const props = defineProps<{ sid: number }>()
const msg = useMessage()
const rows = ref<ConfigFileSet[]>([])
const loading = ref(false)
const show = ref(false)
const label = ref('')
// 本地用 text 便于输入，提交时 base64 编码
const files = ref<{ name: string; text: string; mode: number }[]>([
  { name: '', text: '', mode: 0o644 },
])

async function load() {
  loading.value = true
  try { rows.value = await listConfigSets(props.sid) }
  finally { loading.value = false }
}

async function submit() {
  try {
    const enc = new TextEncoder()
    const toB64 = (s: string) => {
      const bytes = enc.encode(s)
      let bin = ''
      for (let i = 0; i < bytes.length; i++) bin += String.fromCharCode(bytes[i])
      return btoa(bin)
    }
    const payload: ConfigFileItem[] = files.value
      .filter(f => f.name)
      .map(f => ({ name: f.name, content_b64: toB64(f.text), mode: f.mode || 0o644 }))
    await createConfigSet(props.sid, { label: label.value, files: payload })
    msg.success('已创建')
    show.value = false
    label.value = ''
    files.value = [{ name: '', text: '', mode: 0o644 }]
    await load()
  } catch (e: any) { msg.error(e?.message || '失败') }
}

const columns: DataTableColumns<ConfigFileSet> = [
  { title: 'ID', key: 'id', width: 60 },
  { title: 'Label', key: 'label' },
  { title: 'CreatedAt', key: 'created_at', width: 180 },
]

onMounted(load)
</script>

<template>
  <div>
    <NSpace justify="end" style="margin-bottom:8px">
      <NButton type="primary" size="small" @click="show = true">新建 ConfigSet</NButton>
    </NSpace>
    <NDataTable :columns="columns" :data="rows" :loading="loading" size="small" />

    <NModal v-model:show="show" preset="card" title="新建 ConfigSet" style="width:720px">
      <NForm size="small" label-placement="top">
        <NFormItem label="Label">
          <NInput v-model:value="label" />
        </NFormItem>
        <NFormItem label="文件">
          <NDynamicInput
            v-model:value="files"
            :on-create="() => ({ name: '', text: '', mode: 0o644 })"
          >
            <template #default="{ value }">
              <NSpace vertical style="width:100%">
                <NSpace>
                  <NInput v-model:value="value.name" placeholder="path/to/file" style="width:260px" />
                  <NInputNumber v-model:value="value.mode" :show-button="false" style="width:100px" placeholder="mode" />
                </NSpace>
                <NInput v-model:value="value.text" type="textarea" :rows="3" placeholder="文件内容（明文）" />
              </NSpace>
            </template>
          </NDynamicInput>
        </NFormItem>
      </NForm>
      <NSpace justify="end">
        <NButton size="small" @click="show = false">取消</NButton>
        <NButton type="primary" size="small" @click="submit">创建</NButton>
      </NSpace>
    </NModal>
  </div>
</template>
