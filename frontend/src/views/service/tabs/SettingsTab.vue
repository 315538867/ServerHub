<script setup lang="ts">
import { ref, watch } from 'vue'
import { NCard, NSwitch, NSpace, NText, useMessage } from 'naive-ui'
import { setAutoRollback } from '@/api/release'

const props = defineProps<{ sid: number; autoRollback: boolean }>()
const emit = defineEmits<{ (e: 'refresh'): void }>()
const msg = useMessage()
const value = ref(props.autoRollback)

watch(() => props.autoRollback, v => { value.value = v })

async function onChange(v: boolean) {
  try {
    await setAutoRollback(props.sid, v)
    msg.success(v ? '已开启自动回滚' : '已关闭自动回滚')
    emit('refresh')
  } catch (e: any) {
    msg.error(e?.message || '更新失败')
    value.value = !v
  }
}
</script>

<template>
  <NCard size="small" title="自动回滚">
    <NSpace align="center">
      <NSwitch :value="value" @update:value="onChange" />
      <NText depth="3">部署失败时，自动回滚到上一个 active Release（递归深度上限 1）</NText>
    </NSpace>
  </NCard>
</template>
