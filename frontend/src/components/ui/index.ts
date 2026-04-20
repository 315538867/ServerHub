import type { App } from 'vue'
import UiCard from './UiCard.vue'
import UiSection from './UiSection.vue'
import UiTableCard from './UiTableCard.vue'
import StatusDot from './StatusDot.vue'
import StatusTag from './StatusTag.vue'
import EmptyBlock from './EmptyBlock.vue'
import LogOutput from './LogOutput.vue'

export { UiCard, UiSection, UiTableCard, StatusDot, StatusTag, EmptyBlock, LogOutput }
export type { UiStatus } from './StatusDot.vue'

export function registerUi(app: App): void {
  app.component('UiCard', UiCard)
  app.component('UiSection', UiSection)
  app.component('UiTableCard', UiTableCard)
  app.component('StatusDot', StatusDot)
  app.component('StatusTag', StatusTag)
  app.component('EmptyBlock', EmptyBlock)
  app.component('LogOutput', LogOutput)
}
