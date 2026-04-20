<template>
  <div class="ui-empty">
    <t-empty :description="description" />
  </div>
</template>

<script setup lang="ts">
withDefaults(defineProps<{ description?: string }>(), {
  description: '暂无数据',
})
</script>

<style scoped>
.ui-empty {
  padding: var(--ui-space-8) var(--ui-space-6);
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 160px;
}
</style>
