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
  font-family: var(--font-sans);
  font-size: 11px;
  font-weight: var(--fw-medium);
  color: var(--ui-fg-3);
  background: var(--ui-bg-2);
  border: 1px solid var(--ui-border);
  border-radius: 4px;
  line-height: 1;
}
</style>
