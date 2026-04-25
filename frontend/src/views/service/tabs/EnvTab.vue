<script setup lang="ts">
import { onMounted, ref } from 'vue'
import {
  NButton, NDataTable, NForm, NFormItem, NInput, NModal, NSpace,
  NDynamicInput, NCheckbox, useMessage,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { createEnvSet, listEnvSets } from '@/api/release'
import type { EnvVarItem, EnvVarSet } from '@/types/release'

const props = defineProps<{ sid: number }>()
const msg = useMessage()
const rows = ref<EnvVarSet[]>([])
const loading = ref(false)
const show = ref(false)
const label = ref('')
const vars = ref<EnvVarItem[]>([{ key: '', value: '', secret: false }])

async function load() {
  loading.value = true
  try { rows.value = await listEnvSets(props.sid) }
  finally { loading.value = false }
}

async function submit() {
  try {
    await createEnvSet(props.sid, { label: label.value, vars: vars.value.filter(v => v.key) })
    msg.success('已创建')
    show.value = false
    label.value = ''
    vars.value = [{ key: '', value: '', secret: false }]
    await load()
  } catch (e: any) { msg.error(e?.message || '失败') }
}

const columns: DataTableColumns<EnvVarSet> = [
  { title: 'ID', key: 'id', width: 60 },
  { title: 'Label', key: 'label' },
  { title: 'CreatedAt', key: 'created_at', width: 180 },
]

onMounted(load)
</script>

<template>
  <div>
    <NSpace justify="end" style="margin-bottom:8px">
      <NButton type="primary" size="small" @click="show = true">新建 EnvSet</NButton>
    </NSpace>
    <NDataTable :columns="columns" :data="rows" :loading="loading" size="small" />

    <NModal v-model:show="show" preset="card" title="新建 EnvSet" style="width:600px">
      <NForm size="small" label-placement="top">
        <NFormItem label="Label">
          <NInput v-model:value="label" placeholder="e.g. prod-secrets-v1" />
        </NFormItem>
        <NFormItem label="变量">
          <NDynamicInput
            v-model:value="vars"
            :on-create="() => ({ key: '', value: '', secret: false })"
          >
            <template #default="{ value }">
              <NSpace style="width:100%">
                <NInput v-model:value="value.key" placeholder="KEY" style="width:160px" />
                <NInput v-model:value="value.value" placeholder="value" style="width:240px" />
                <NCheckbox v-model:checked="value.secret">secret</NCheckbox>
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
