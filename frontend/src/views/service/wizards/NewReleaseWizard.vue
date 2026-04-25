<script setup lang="ts">
import { onMounted, ref } from 'vue'
import {
  NButton, NForm, NFormItem, NInput, NSelect, NSpace, NSteps, NStep,
  NText, useMessage,
} from 'naive-ui'
import {
  createRelease, listArtifacts, listConfigSets, listEnvSets,
} from '@/api/release'
import type { Artifact, ConfigFileSet, EnvVarSet } from '@/types/release'

const props = defineProps<{ sid: number }>()
const emit = defineEmits<{ (e: 'done'): void }>()
const msg = useMessage()

const current = ref(1)
const label = ref('')
const note = ref('')
const artifactId = ref<number | null>(null)
const envSetId = ref<number | null>(null)
const configSetId = ref<number | null>(null)
const startType = ref<'docker' | 'docker-compose' | 'native' | 'static'>('docker')
const image = ref('')
const cmd = ref('')
const fileName = ref('docker-compose.yml')

const artifacts = ref<Artifact[]>([])
const envSets = ref<EnvVarSet[]>([])
const configSets = ref<ConfigFileSet[]>([])

onMounted(async () => {
  try {
    [artifacts.value, envSets.value, configSets.value] = await Promise.all([
      listArtifacts(props.sid),
      listEnvSets(props.sid),
      listConfigSets(props.sid),
    ])
  } catch (e: any) {
    msg.error(e?.message || '加载失败')
  }
})

function buildStartSpec(): Record<string, unknown> {
  switch (startType.value) {
    case 'docker': return { type: 'docker', image: image.value, cmd: cmd.value }
    case 'docker-compose': return { type: 'docker-compose', file_name: fileName.value }
    case 'native': return { type: 'native', cmd: cmd.value }
    case 'static': return { type: 'static' }
  }
}

async function submit() {
  if (!artifactId.value) { msg.warning('请选择 Artifact'); return }
  try {
    const rel = await createRelease(props.sid, {
      label: label.value || undefined,
      artifact_id: artifactId.value,
      env_set_id: envSetId.value,
      config_set_id: configSetId.value,
      start_spec: buildStartSpec(),
      note: note.value,
    })
    msg.success(`Release #${rel.id} ${rel.label} 已创建（draft，可在列表中应用）`)
    emit('done')
  } catch (e: any) {
    msg.error(e?.message || '创建失败')
  }
}

const artifactOptions = () => artifacts.value.map(a => ({
  label: `#${a.id} ${a.provider}${a.ref ? ' · ' + a.ref : ''}`,
  value: a.id,
}))
const envOptions = () => envSets.value.map(e => ({ label: `#${e.id} ${e.label || '(unnamed)'}`, value: e.id }))
const cfgOptions = () => configSets.value.map(c => ({ label: `#${c.id} ${c.label || '(unnamed)'}`, value: c.id }))
</script>

<template>
  <div>
    <NSteps :current="current" size="small" style="margin-bottom:12px">
      <NStep title="Artifact" />
      <NStep title="Env" />
      <NStep title="Config" />
      <NStep title="Start" />
      <NStep title="确认" />
    </NSteps>

    <NForm label-placement="top" size="small">
      <template v-if="current === 1">
        <NFormItem label="Artifact (制品)">
          <NSelect v-model:value="artifactId" :options="artifactOptions()" placeholder="选择已存在的 Artifact" />
        </NFormItem>
        <NText depth="3" style="font-size:12px">
          没有现成 Artifact？请先在"制品"Tab 上传或声明（M1 该 Tab 在占位，可改用后端 API 创建）
        </NText>
      </template>

      <template v-if="current === 2">
        <NFormItem label="EnvVarSet (可选)">
          <NSelect v-model:value="envSetId" :options="envOptions()" clearable placeholder="不选则不注入环境变量" />
        </NFormItem>
      </template>

      <template v-if="current === 3">
        <NFormItem label="ConfigFileSet (可选)">
          <NSelect v-model:value="configSetId" :options="cfgOptions()" clearable placeholder="不选则不下发配置" />
        </NFormItem>
      </template>

      <template v-if="current === 4">
        <NFormItem label="Service 启动方式">
          <NSelect
            v-model:value="startType"
            :options="[
              { label: 'docker', value: 'docker' },
              { label: 'docker-compose', value: 'docker-compose' },
              { label: 'native', value: 'native' },
              { label: 'static', value: 'static' },
            ]"
          />
        </NFormItem>
        <NFormItem v-if="startType === 'docker'" label="镜像 (留空=用 Artifact.Ref)">
          <NInput v-model:value="image" placeholder="e.g. redis:7-alpine" />
        </NFormItem>
        <NFormItem v-if="startType === 'docker'" label="附加 cmd">
          <NInput v-model:value="cmd" placeholder="容器启动参数" />
        </NFormItem>
        <NFormItem v-if="startType === 'docker-compose'" label="Compose 文件名">
          <NInput v-model:value="fileName" />
        </NFormItem>
        <NFormItem v-if="startType === 'native'" label="启动命令">
          <NInput v-model:value="cmd" placeholder="e.g. ./hello" />
        </NFormItem>
      </template>

      <template v-if="current === 5">
        <NFormItem label="Label (留空自动生成)">
          <NInput v-model:value="label" placeholder="e.g. 2026-04-24-1" />
        </NFormItem>
        <NFormItem label="备注">
          <NInput v-model:value="note" type="textarea" :rows="2" />
        </NFormItem>
        <pre style="background:#f5f5f5;padding:8px;border-radius:4px;font-size:12px">{{
          JSON.stringify({
            artifact_id: artifactId, env_set_id: envSetId,
            config_set_id: configSetId, start_spec: buildStartSpec(),
          }, null, 2)
        }}</pre>
      </template>
    </NForm>

    <NSpace justify="end" style="margin-top:12px">
      <NButton v-if="current > 1" size="small" @click="current--">上一步</NButton>
      <NButton v-if="current < 5" type="primary" size="small" @click="current++">下一步</NButton>
      <NButton v-else type="primary" size="small" @click="submit">创建</NButton>
    </NSpace>
  </div>
</template>
