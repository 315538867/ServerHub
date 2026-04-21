<template>
  <n-config-provider
    :theme="themeStore.isDark ? darkTheme : null"
    :theme-overrides="themeOverrides"
    :locale="zhCN"
    :date-locale="dateZhCN"
  >
    <n-message-provider>
      <n-dialog-provider>
        <n-notification-provider>
          <router-view />
        </n-notification-provider>
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import {
  NConfigProvider, NMessageProvider, NDialogProvider, NNotificationProvider,
  darkTheme, zhCN, dateZhCN,
  type GlobalThemeOverrides,
} from 'naive-ui'
import { useThemeStore } from '@/stores/theme'

const themeStore = useThemeStore()

const themeOverrides = computed<GlobalThemeOverrides>(() => {
  const dark = themeStore.isDark
  return {
    common: {
      primaryColor: '#10B981',
      primaryColorHover: dark ? '#34D399' : '#059669',
      primaryColorPressed: '#047857',
      primaryColorSuppl: '#10B981',

      infoColor: '#3B82F6',
      infoColorHover: '#60A5FA',
      successColor: '#24B47E',
      warningColor: '#F59E0B',
      errorColor: '#EF4444',

      borderRadius: '6px',
      borderRadiusSmall: '4px',

      fontFamily: "Inter, -apple-system, BlinkMacSystemFont, 'PingFang SC', 'Helvetica Neue', 'Microsoft YaHei', sans-serif",
      fontFamilyMono: "'JetBrains Mono', 'SF Mono', Menlo, Consolas, monospace",

      fontSize: '14px',
      fontSizeSmall: '13px',
      fontSizeMini: '12px',

      bodyColor: dark ? '#0A0A0A' : '#FAFAFA',
      cardColor: dark ? '#111113' : '#FFFFFF',
      modalColor: dark ? '#111113' : '#FFFFFF',
      popoverColor: dark ? '#18181B' : '#FFFFFF',
      tableColor: dark ? '#111113' : '#FFFFFF',
      tableColorStriped: dark ? '#18181B' : '#FAFAFA',
      tableHeaderColor: dark ? '#18181B' : '#FAFAFA',

      dividerColor: dark ? '#27272A' : '#E4E4E7',
      borderColor: dark ? '#27272A' : '#E4E4E7',

      textColorBase: dark ? '#FAFAFA' : '#18181B',
      textColor1: dark ? '#FAFAFA' : '#18181B',
      textColor2: dark ? '#A1A1AA' : '#52525B',
      textColor3: dark ? '#71717A' : '#71717A',
      textColorDisabled: dark ? '#52525B' : '#A1A1AA',
      placeholderColor: dark ? '#71717A' : '#A1A1AA',

      boxShadow1: '0 1px 2px rgba(9,9,11,0.04), 0 0 0 1px rgba(9,9,11,0.04)',
      boxShadow2: '0 4px 12px rgba(9,9,11,0.08), 0 2px 4px rgba(9,9,11,0.04)',
      boxShadow3: '0 12px 28px rgba(9,9,11,0.12)',

      heightTiny: '24px',
      heightSmall: '28px',
      heightMedium: '32px',
      heightLarge: '36px',
    },
    Button: {
      fontWeight: '500',
      textColorPrimary: '#FFFFFF',
      textColorHoverPrimary: '#FFFFFF',
      textColorPressedPrimary: '#FFFFFF',
      textColorFocusPrimary: '#FFFFFF',
      borderRadiusMedium: '6px',
    },
    Card: {
      borderRadius: '10px',
      paddingMedium: '20px',
      titleFontWeight: '600',
    },
    DataTable: {
      thColor: dark ? '#18181B' : '#FAFAFA',
      thColorHover: dark ? '#27272A' : '#F4F4F5',
      thFontWeight: '500',
      thTextColor: dark ? '#A1A1AA' : '#71717A',
      tdColorHover: dark ? '#18181B' : '#FAFAFA',
      borderRadius: '10px',
    },
    Input: {
      borderRadius: '6px',
      heightMedium: '32px',
    },
    Select: {
      peers: { InternalSelection: { borderRadius: '6px' } },
    },
    Tag: {
      borderRadius: '4px',
      heightMedium: '22px',
      fontWeightStrong: '500',
    },
    Tabs: {
      tabFontWeightActive: '600',
      tabFontSizeMedium: '13px',
      tabGapMediumLine: '24px',
    },
  }
})
</script>
