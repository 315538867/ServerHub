<template>
  <kbd class="ui-kbd"><slot /></kbd>
</template>

<style scoped>
.ui-kbd {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 18px;
  height: 18px;
  padding: 0 5px;
  font-family: var(--ui-font-sans);
  font-size: 10.5px;
  font-weight: var(--ui-fw-medium);
  color: var(--ui-fg-3);
  background: var(--ui-bg-surface);
  border: 1px solid var(--ui-border);
  border-bottom-width: 2px;
  border-radius: var(--ui-radius-xs);
  line-height: 1;
}
</style>
