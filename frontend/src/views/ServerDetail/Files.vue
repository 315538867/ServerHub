<template>
  <div class="fs-page">
    <UiCard padding="none">
      <div class="fs-head">
        <div class="fs-crumbs">
          <span class="crumb" :class="{ 'is-root': true }" @click="navigateTo('/')">根目录</span>
          <template v-for="(seg, i) in pathSegments" :key="i">
            <ChevronRight :size="12" class="crumb-sep" />
            <span class="crumb" @click="navigateTo('/' + pathSegments.slice(0, i + 1).join('/'))">{{ seg }}</span>
          </template>
        </div>
        <div class="fs-actions">
          <UiButton variant="secondary" size="sm" @click="triggerUpload">
            <template #icon><Upload :size="14" /></template>
            上传
          </UiButton>
          <UiButton variant="secondary" size="sm" @click="openMkdir">
            <template #icon><FolderPlus :size="14" /></template>
            新建目录
          </UiButton>
          <UiButton variant="secondary" size="sm" :loading="loading" @click="reload">
            <template #icon><RefreshCw :size="14" /></template>
            刷新
          </UiButton>
        </div>
      </div>
      <div class="fs-body">
        <NDataTable
          :columns="fileColumns"
          :data="entries"
          :loading="loading"
          :row-key="(row: FileEntry) => row.name"
          size="small"
          :bordered="false"
          :row-props="rowProps"
        />
      </div>
    </UiCard>

    <input ref="uploadInput" type="file" multiple style="display:none" @change="onUploadChange" />

    <NModal v-model:show="mkdirVisible" preset="card" title="新建目录" style="width: 400px" :bordered="false" @after-leave="mkdirName = ''">
      <NInput v-model:value="mkdirName" placeholder="目录名" @keydown.enter="confirmMkdir" />
      <template #footer>
        <div class="modal-foot">
          <UiButton variant="secondary" size="sm" @click="mkdirVisible = false">取消</UiButton>
          <UiButton variant="primary" size="sm" @click="confirmMkdir">创建</UiButton>
        </div>
      </template>
    </NModal>

    <NModal v-model:show="renameVisible" preset="card" title="重命名 / 移动" style="width: 440px" :bordered="false">
      <NInput v-model:value="renameTo" placeholder="新路径" />
      <template #footer>
        <div class="modal-foot">
          <UiButton variant="secondary" size="sm" @click="renameVisible = false">取消</UiButton>
          <UiButton variant="primary" size="sm" @click="confirmRename">确认</UiButton>
        </div>
      </template>
    </NModal>

    <NModal v-model:show="chmodVisible" preset="card" title="修改权限" style="width: 400px" :bordered="false">
      <NInput v-model:value="chmodMode" placeholder="如 0644 或 755" />
      <template #footer>
        <div class="modal-foot">
          <UiButton variant="secondary" size="sm" @click="chmodVisible = false">取消</UiButton>
          <UiButton variant="primary" size="sm" @click="confirmChmod">确认</UiButton>
        </div>
      </template>
    </NModal>

    <NModal
      v-model:show="editVisible"
      preset="card"
      :title="`编辑 — ${editPath}`"
      style="width: 800px"
      :bordered="false"
      :mask-closable="false"
      @after-leave="destroyEditor"
    >
      <div ref="editorEl" class="code-editor" />
      <template #footer>
        <div class="modal-foot">
          <UiButton variant="secondary" size="sm" @click="editVisible = false">取消</UiButton>
          <UiButton variant="primary" size="sm" :loading="saving" @click="saveEdit">保存</UiButton>
        </div>
      </template>
    </NModal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onMounted, onBeforeUnmount, h } from 'vue'
import { useRoute } from 'vue-router'
import { NDataTable, NModal, NInput, NPopconfirm, useMessage } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { RefreshCw, Upload, FolderPlus, Folder, File as FileIcon, ChevronRight } from 'lucide-vue-next'
import { EditorView, basicSetup } from 'codemirror'
import { EditorState } from '@codemirror/state'
import { json } from '@codemirror/lang-json'
import { yaml } from '@codemirror/lang-yaml'
import { javascript } from '@codemirror/lang-javascript'
import { oneDark } from '@codemirror/theme-one-dark'
import { useAuthStore } from '@/stores/auth'
import {
  listFiles, getFileContent, putFileContent, downloadFile,
  uploadFile, mkdir, deleteFile, renameFile, chmod,
} from '@/api/files'
import type { FileEntry } from '@/api/files'
import UiCard from '@/components/ui/UiCard.vue'
import UiButton from '@/components/ui/UiButton.vue'

const route = useRoute()
const auth = useAuthStore()
const message = useMessage()
const serverId = computed(() => Number(route.params.serverId))
const currentPath = ref('/')
const entries = ref<FileEntry[]>([])
const loading = ref(false)
const pathSegments = computed(() => currentPath.value.split('/').filter(Boolean))

const editableExts = new Set([
  'txt', 'md', 'json', 'yaml', 'yml', 'conf', 'cfg', 'ini',
  'sh', 'bash', 'zsh', 'env', 'toml', 'xml', 'html', 'htm',
  'js', 'ts', 'css', 'sql', 'log', 'nginx', 'htaccess',
])

function isEditable(name: string) {
  return editableExts.has(name.split('.').pop()?.toLowerCase() ?? '')
}
function getLang(filename: string) {
  const ext = filename.split('.').pop()?.toLowerCase() ?? ''
  if (ext === 'json') return [json()]
  if (ext === 'yaml' || ext === 'yml') return [yaml()]
  if (['js', 'ts', 'mjs'].includes(ext)) return [javascript()]
  return []
}
function fullPath(name: string) {
  return (currentPath.value.endsWith('/') ? currentPath.value : currentPath.value + '/') + name
}
function formatSize(bytes: number) {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1048576) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1048576).toFixed(1)} MB`
}

const fileColumns = computed<DataTableColumns<FileEntry>>(() => [
  {
    title: '名称', key: 'name', minWidth: 240, ellipsis: { tooltip: true },
    render: (row) => h('div', { class: 'fs-name' }, [
      h(row.is_dir ? Folder : FileIcon, { size: 14, class: row.is_dir ? 'ico dir' : 'ico file' }),
      h('span', null, row.name),
    ]),
  },
  {
    title: '大小', key: 'size', width: 100,
    render: (row) => h('span', { class: 'mono' }, row.is_dir ? '—' : formatSize(row.size)),
  },
  {
    title: '权限', key: 'mode', width: 130,
    render: (row) => h('span', { class: 'mono perm' }, row.mode),
  },
  { title: '修改时间', key: 'mod_time', minWidth: 160, ellipsis: { tooltip: true } },
  {
    title: '操作', key: 'ops', width: 300, fixed: 'right' as const,
    render: (row) => h('div', { class: 'cell-ops' }, [
      !row.is_dir ? h(UiButton, { variant: 'ghost', size: 'sm', onClick: () => downloadEntry(row) }, () => '下载') : null,
      !row.is_dir && isEditable(row.name) ? h(UiButton, { variant: 'ghost', size: 'sm', onClick: () => openEdit(row) }, () => '编辑') : null,
      h(UiButton, { variant: 'ghost', size: 'sm', onClick: () => openRename(row) }, () => '重命名'),
      h(UiButton, { variant: 'ghost', size: 'sm', onClick: () => openChmod(row) }, () => '权限'),
      h(NPopconfirm, {
        onPositiveClick: () => deleteEntry(row),
        positiveText: '删除', negativeText: '取消',
      }, {
        trigger: () => h(UiButton, { variant: 'ghost', size: 'sm' },
          () => h('span', { class: 'text-danger' }, '删除')),
        default: () => `确认删除 ${row.name}？`,
      }),
    ]),
  },
])

function rowProps(row: FileEntry) {
  return {
    style: row.is_dir ? 'cursor: pointer' : '',
    ondblclick: () => { if (row.is_dir) navigateTo(fullPath(row.name)) },
  }
}

async function navigateTo(path: string) { currentPath.value = path || '/'; await reload() }

async function reload() {
  loading.value = true
  try {
    const res = await listFiles(serverId.value, currentPath.value)
    entries.value = res.entries ?? []
  } catch { message.error('读取目录失败') }
  finally { loading.value = false }
}

const uploadInput = ref<HTMLInputElement>()
function triggerUpload() { uploadInput.value?.click() }
async function onUploadChange(e: Event) {
  const files = (e.target as HTMLInputElement).files
  if (!files) return
  for (const file of Array.from(files)) {
    try { await uploadFile(serverId.value, currentPath.value, file); message.success(`${file.name} 上传成功`) }
    catch { message.error(`${file.name} 上传失败`) }
  }
  ;(e.target as HTMLInputElement).value = ''
  await reload()
}

async function downloadEntry(row: FileEntry) {
  try { await downloadFile(serverId.value, fullPath(row.name), auth.token) }
  catch { message.error('下载失败') }
}

async function deleteEntry(row: FileEntry) {
  try { await deleteFile(serverId.value, fullPath(row.name)); message.success('已删除'); await reload() }
  catch { message.error('删除失败') }
}

const mkdirVisible = ref(false)
const mkdirName = ref('')
function openMkdir() { mkdirName.value = ''; mkdirVisible.value = true }
async function confirmMkdir() {
  if (!mkdirName.value.trim()) return
  try { await mkdir(serverId.value, fullPath(mkdirName.value.trim())); message.success('目录已创建'); mkdirVisible.value = false; await reload() }
  catch { message.error('创建失败') }
}

const renameVisible = ref(false)
const renameFrom = ref('')
const renameTo = ref('')
function openRename(row: FileEntry) { renameFrom.value = fullPath(row.name); renameTo.value = renameFrom.value; renameVisible.value = true }
async function confirmRename() {
  try { await renameFile(serverId.value, renameFrom.value, renameTo.value); message.success('重命名成功'); renameVisible.value = false; await reload() }
  catch { message.error('重命名失败') }
}

const chmodVisible = ref(false)
const chmodPath = ref('')
const chmodMode = ref('')
function openChmod(row: FileEntry) { chmodPath.value = fullPath(row.name); chmodMode.value = ''; chmodVisible.value = true }
async function confirmChmod() {
  if (!chmodMode.value.trim()) return
  try { await chmod(serverId.value, chmodPath.value, chmodMode.value.trim()); message.success('权限已修改'); chmodVisible.value = false; await reload() }
  catch { message.error('修改权限失败') }
}

const editVisible = ref(false)
const editPath = ref('')
const saving = ref(false)
const editorEl = ref<HTMLDivElement>()
let editorView: EditorView | null = null

async function openEdit(row: FileEntry) {
  editPath.value = fullPath(row.name); editVisible.value = true
  await nextTick()
  try {
    const res = await getFileContent(serverId.value, editPath.value)
    editorView?.destroy()
    editorView = new EditorView({
      state: EditorState.create({ doc: res.content, extensions: [basicSetup, oneDark, ...getLang(row.name)] }),
      parent: editorEl.value!,
    })
  } catch (e: any) { message.error(e?.response?.data?.msg ?? '读取文件失败'); editVisible.value = false }
}

async function saveEdit() {
  if (!editorView) return
  saving.value = true
  try {
    await putFileContent(serverId.value, editPath.value, editorView.state.doc.toString())
    message.success('保存成功'); editVisible.value = false
  } catch { message.error('保存失败') }
  finally { saving.value = false }
}

function destroyEditor() { editorView?.destroy(); editorView = null }

onMounted(() => reload())
onBeforeUnmount(() => editorView?.destroy())
</script>

<style scoped>
.fs-page { padding: var(--space-6); display: flex; flex-direction: column; gap: var(--space-4); }

.fs-head {
  display: flex; align-items: center; justify-content: space-between;
  padding: var(--space-3) var(--space-4);
  border-bottom: 1px solid var(--ui-border);
  gap: var(--space-3); flex-wrap: wrap;
}

.fs-crumbs {
  display: inline-flex; align-items: center;
  gap: var(--space-1);
  font-size: var(--fs-sm); color: var(--ui-fg-2);
  min-width: 0; flex: 1;
}
.crumb {
  cursor: pointer;
  padding: 2px 6px;
  border-radius: var(--radius-sm);
  transition: background var(--dur-fast) var(--ease), color var(--dur-fast) var(--ease);
}
.crumb:hover { background: var(--ui-bg-2); color: var(--ui-fg); }
.crumb-sep { color: var(--ui-fg-4); flex-shrink: 0; }

.fs-actions { display: flex; gap: var(--space-2); flex-shrink: 0; }

.fs-body { padding: var(--space-4); }

.modal-foot { display: flex; justify-content: flex-end; gap: var(--space-2); }

.code-editor {
  height: 60vh; overflow: auto;
  font-size: 13px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--ui-border);
}
:deep(.cm-editor) { height: 100%; }
:deep(.cm-scroller) { overflow: auto; }

:deep(.fs-name) {
  display: inline-flex; align-items: center; gap: var(--space-2);
  user-select: none;
}
:deep(.fs-name .ico.dir) { color: var(--ui-brand); }
:deep(.fs-name .ico.file) { color: var(--ui-fg-4); }

:deep(.mono) {
  font-family: var(--font-mono);
  font-size: var(--fs-xs);
}
:deep(.perm) { color: var(--ui-fg-3); }

:deep(.cell-ops) { display: inline-flex; gap: var(--space-1); }
:deep(.text-danger) { color: var(--ui-danger-fg); }
</style>
