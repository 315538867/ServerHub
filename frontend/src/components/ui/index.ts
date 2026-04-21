import type { App } from 'vue'
import UiCard from './UiCard.vue'
import UiSection from './UiSection.vue'
import UiTableCard from './UiTableCard.vue'
import UiButton from './UiButton.vue'
import UiBadge from './UiBadge.vue'
import UiKbd from './UiKbd.vue'
import UiToolbar from './UiToolbar.vue'
import UiPageHeader from './UiPageHeader.vue'
import UiStatCard from './UiStatCard.vue'
import UiSparkline from './UiSparkline.vue'
import UiIconButton from './UiIconButton.vue'
import UiThemeToggle from './UiThemeToggle.vue'
import UiTabs from './UiTabs.vue'
import UiSkeleton from './UiSkeleton.vue'
import UiStateBanner from './UiStateBanner.vue'
import StatusDot from './StatusDot.vue'
import StatusTag from './StatusTag.vue'
import EmptyBlock from './EmptyBlock.vue'
import LogOutput from './LogOutput.vue'

export {
  UiCard, UiSection, UiTableCard,
  UiButton, UiBadge, UiKbd,
  UiToolbar, UiPageHeader, UiStatCard, UiSparkline,
  UiIconButton, UiThemeToggle,
  UiTabs, UiSkeleton, UiStateBanner,
  StatusDot, StatusTag, EmptyBlock, LogOutput,
}
export type { UiStatus } from './StatusDot.vue'

export function registerUi(app: App): void {
  app.component('UiCard', UiCard)
  app.component('UiSection', UiSection)
  app.component('UiTableCard', UiTableCard)
  app.component('UiButton', UiButton)
  app.component('UiBadge', UiBadge)
  app.component('UiKbd', UiKbd)
  app.component('UiToolbar', UiToolbar)
  app.component('UiPageHeader', UiPageHeader)
  app.component('UiStatCard', UiStatCard)
  app.component('UiSparkline', UiSparkline)
  app.component('UiIconButton', UiIconButton)
  app.component('UiThemeToggle', UiThemeToggle)
  app.component('UiTabs', UiTabs)
  app.component('UiSkeleton', UiSkeleton)
  app.component('UiStateBanner', UiStateBanner)
  app.component('StatusDot', StatusDot)
  app.component('StatusTag', StatusTag)
  app.component('EmptyBlock', EmptyBlock)
  app.component('LogOutput', LogOutput)
}
