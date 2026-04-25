<template>
  <div class="net-page">
    <UiSection>
      <template #title>
        <span class="net-title"><NetworkIcon :size="16" /> 网络入口</span>
      </template>
      <template #extra>
        <UiButton variant="secondary" size="sm" :loading="loading" @click="reload">
          <template #icon><RefreshCw :size="14" /></template>
          刷新
        </UiButton>
        <UiButton variant="primary" size="sm" :loading="saving" @click="save">保存</UiButton>
      </template>

      <UiCard padding="md">
        <p class="net-hint">
          loopback 由系统自动维护、用户不能编辑。private/vpn 必填 network_id（同 ID 视为互通），
          tunnel 必填 reachable_from（哪些 server 能用此地址），public 永远是兜底。
        </p>

        <NDataTable
          :columns="columns"
          :data="rows"
          :bordered="false"
          size="small"
          class="net-table"
        />

        <div class="net-actions">
          <UiButton variant="secondary" size="sm" @click="addRow('private')">+ 内网</UiButton>
          <UiButton variant="secondary" size="sm" @click="addRow('vpn')">+ VPN</UiButton>
          <UiButton variant="secondary" size="sm" @click="addRow('tunnel')">+ 隧道</UiButton>
          <UiButton variant="secondary" size="sm" @click="addRow('public')">+ 公网</UiButton>
        </div>
      </UiCard>
    </UiSection>
  </div>
</template>

<script setup lang="ts">
import { h, ref, onMounted, computed } from 'vue'
import { useRoute } from 'vue-router'
import { NDataTable, NInput, NInputNumber, NSelect, NDynamicTags, NTag, NPopconfirm, useMessage } from 'naive-ui'
import { Network as NetworkIcon, RefreshCw, Trash2 } from 'lucide-vue-next'
import { getServerNetworks, updateServerNetworks } from '@/api/servers'
import type { Network, NetworkKind } from '@/types/api'
import UiSection from '@/components/ui/UiSection.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiButton from '@/components/ui/UiButton.vue'

const route = useRoute()
const serverId = computed(() => Number(route.params.serverId))
const message = useMessage()

const rows = ref<Network[]>([])
const loading = ref(false)
const saving = ref(false)

const kindOptions = [
  { label: 'loopback', value: 'loopback', disabled: true },
  { label: '内网 private', value: 'private' },
  { label: 'VPN', value: 'vpn' },
  { label: '隧道 tunnel', value: 'tunnel' },
  { label: '公网 public', value: 'public' },
]

const defaultPriority: Record<NetworkKind, number> = {
  loopback: 0, private: 10, vpn: 20, tunnel: 30, public: 100,
}

async function reload() {
  loading.value = true
  try {
    const r = await getServerNetworks(serverId.value)
    rows.value = r.networks ?? []
  } catch (e: any) {
    message.error('加载失败: ' + (e?.message ?? e))
  } finally {
    loading.value = false
  }
}

function addRow(kind: NetworkKind) {
  rows.value.push({
    kind,
    network_id: kind === 'public' ? 'public' : '',
    address: '',
    priority: defaultPriority[kind],
    reachable_from: kind === 'tunnel' ? [] : undefined,
    label: '',
  })
}

function removeRow(idx: number) {
  rows.value.splice(idx, 1)
}

async function save() {
  // 客户端浅校验，详细校验由后端 BeforeSave 给
  for (const n of rows.value) {
    if (n.kind === 'loopback') continue
    if (!n.address.trim()) {
      message.error('address 不能为空')
      return
    }
    if ((n.kind === 'private' || n.kind === 'vpn') && !n.network_id.trim()) {
      message.error(`${n.kind} 必须填 network_id`)
      return
    }
    if (n.kind === 'tunnel' && (!n.reachable_from || n.reachable_from.length === 0)) {
      message.error('tunnel 必须填 reachable_from（server id 列表）')
      return
    }
  }
  saving.value = true
  try {
    const r = await updateServerNetworks(serverId.value, rows.value)
    rows.value = r.networks ?? []
    message.success('已保存')
  } catch (e: any) {
    message.error('保存失败: ' + (e?.message ?? e))
  } finally {
    saving.value = false
  }
}

const columns = [
  {
    title: 'Kind',
    key: 'kind',
    width: 130,
    render(row: Network) {
      if (row.kind === 'loopback') {
        return h(NTag, { type: 'default', size: 'small' }, { default: () => 'loopback' })
      }
      return h(NSelect, {
        value: row.kind,
        size: 'small',
        options: kindOptions.filter(o => !o.disabled),
        onUpdateValue: (v: NetworkKind) => {
          row.kind = v
          if (v === 'public') row.network_id = 'public'
          row.priority = defaultPriority[v]
          if (v === 'tunnel' && !row.reachable_from) row.reachable_from = []
        },
      })
    },
  },
  {
    title: 'NetworkID',
    key: 'network_id',
    render(row: Network) {
      const ro = row.kind === 'loopback' || row.kind === 'public'
      return h(NInput, {
        value: row.network_id,
        size: 'small',
        readonly: ro,
        placeholder: ro ? '' : '同 ID 视为互通，例：lan-A',
        onUpdateValue: (v: string) => { row.network_id = v },
      })
    },
  },
  {
    title: 'Address',
    key: 'address',
    render(row: Network) {
      return h(NInput, {
        value: row.address,
        size: 'small',
        readonly: row.kind === 'loopback',
        placeholder: row.kind === 'tunnel' ? '127.0.0.1:7000' : 'IP / 域名',
        onUpdateValue: (v: string) => { row.address = v },
      })
    },
  },
  {
    title: 'Priority',
    key: 'priority',
    width: 110,
    render(row: Network) {
      return h(NInputNumber, {
        value: row.priority,
        size: 'small',
        min: 0,
        showButton: false,
        disabled: row.kind === 'loopback',
        onUpdateValue: (v: number | null) => { row.priority = v ?? 0 },
      })
    },
  },
  {
    title: 'Reachable From',
    key: 'reachable_from',
    width: 200,
    render(row: Network) {
      if (row.kind !== 'tunnel') return h('span', { class: 'text-muted' }, '—')
      return h(NDynamicTags, {
        value: (row.reachable_from ?? []).map(String),
        size: 'small',
        onUpdateValue: (vs: string[]) => {
          row.reachable_from = vs.map(s => Number(s)).filter(n => Number.isFinite(n) && n > 0)
        },
      })
    },
  },
  {
    title: 'Label',
    key: 'label',
    render(row: Network) {
      return h(NInput, {
        value: row.label ?? '',
        size: 'small',
        placeholder: 'UI 显示名',
        onUpdateValue: (v: string) => { row.label = v },
      })
    },
  },
  {
    title: '',
    key: 'op',
    width: 60,
    render(row: Network, idx: number) {
      if (row.kind === 'loopback') return null
      return h(NPopconfirm, {
        onPositiveClick: () => removeRow(idx),
      }, {
        trigger: () => h(UiButton, { variant: 'ghost', size: 'sm' }, {
          icon: () => h(Trash2, { size: 14 }),
        }),
        default: () => '删除该条？',
      })
    },
  },
]

onMounted(reload)
</script>

<style scoped>
.net-page {
  padding: var(--space-3);
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  min-height: 0;
  overflow: auto;
}
.net-title {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.net-hint {
  color: var(--ui-text-muted);
  font-size: 13px;
  margin: 0 0 var(--space-2) 0;
}
.net-table {
  margin-bottom: var(--space-2);
}
.net-actions {
  display: flex;
  gap: var(--space-2);
}
.text-muted {
  color: var(--ui-text-muted);
}
</style>
