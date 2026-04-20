<template>
  <div class="page-container">
    <div class="section-block">
      <!-- 路径面包屑 + 操作按钮 -->
      <div class="toolbar">
        <t-breadcrumb class="path-breadcrumb">
          <t-breadcrumb-item @click="navigateTo('/')">根目录</t-breadcrumb-item>
          <t-breadcrumb-item
            v-for="(seg, i) in pathSegments"
            :key="i"
            @click="navigateTo('/' + pathSegments.slice(0, i + 1).join('/'))"
          >{{ seg }}</t-breadcrumb-item>
        </t-breadcrumb>
        <div class="toolbar-right">
          <t-button size="small" variant="outline" @click="triggerUpload">
            <template #icon><upload-icon /></template>
            上传文件
          </t-button>
          <t-button size="small" variant="outline" @click="openMkdir">
            <template #icon><folder-add-icon /></template>
            新建目录
          </t-button>
          <t-button size="small" variant="outline" :loading="loading" @click="reload">
            <template #icon><refresh-icon /></template>
            刷新
          </t-button>
        </div>
      </div>

      <!-- 文件列表 -->
      <t-table
        :data="entries"
        :columns="fileColumns"
        :loading="loading"
        row-key="name"
        bordered
        size="small"
        empty="目录为空"
        @row-dblclick="onRowDblClick"
      >
        <template #name="{ row }">
          <div class="file-name-cell">
            <folder-icon v-if="row.is_dir" class="file-icon dir" />
            <file-icon v-else class="file-icon file" />
            <span>{{ row.name }}</span>
          </div>
        </template>
        <template #size="{ row }">
          <span class="mono-text">{{ row.is_dir ? '—' : formatSize(row.size) }}</span>
        </template>
        <template #mode="{ row }">
          <span class="mono-text perm">{{ row.mode }}</span>
        </template>
        <template #operations="{ row }">
          <t-space size="small">
            <t-button v-if="!row.is_dir" size="small" variant="text" @click="downloadEntry(row)">下载</t-button>
            <t-button v-if="!row.is_dir && isEditable(row.name)" size="small" variant="text" @click="openEdit(row)">编辑</t-button>
            <t-button size="small" variant="text" @click="openRename(row)">重命名</t-button>
            <t-button size="small" variant="text" @click="openChmod(row)">权限</t-button>
            <t-popconfirm :content="`确认删除 ${row.name}？`" @confirm="deleteEntry(row)">
              <t-button theme="danger" size="small" variant="text">删除</t-button>
            </t-popconfirm>
          </t-space>
        </template>
      </t-table>
    </div>

    <input ref="uploadInput" type="file" multiple style="display:none" @change="onUploadChange" />

    <!-- 新建目录 -->
    <t-dialog
      v-model:visible="mkdirVisible"
      header="新建目录"
      width="400px"
      confirm-btn="创建"
      @confirm="confirmMkdir"
      @closed="mkdirName = ''"
    >
      <t-input v-model="mkdirName" placeholder="目录名" @keydown.enter="confirmMkdir" />
    </t-dialog>

    <!-- 重命名 -->
    <t-dialog
      v-model:visible="renameVisible"
      header="重命名 / 移动"
      width="400px"
      confirm-btn="确认"
      @confirm="confirmRename"
    >
      <t-input v-model="renameTo" placeholder="新路径" />
    </t-dialog>

    <!-- 修改权限 -->
    <t-dialog
      v-model:visible="chmodVisible"
      header="修改权限"
      width="400px"
      confirm-btn="确认"
      @confirm="confirmChmod"
    >
      <t-input v-model="chmodMode" placeholder="如 0644 或 755" />
    </t-dialog>

    <!-- 编辑文件 (CodeMirror) -->
    <t-dialog
      v-model:visible="editVisible"
      :header="`编辑 — ${editPath}`"
      width="800px"
      :close-on-overlay-click="false"
      :confirm-btn="{ content: '保存', loading: saving }"
      @confirm="saveEdit"
      @closed="destroyEditor"
    >
      <div ref="editorEl" class="code-editor" />
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onMounted, onBeforeUnmount } from 'vue'
import { useRoute } from 'vue-router'
import { RefreshIcon, UploadIcon, FolderAddIcon, FolderIcon, FileIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'
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

const route = useRoute()
const auth = useAuthStore()
const serverId = computed(() => Number(route.params.serverId))
const currentPath = ref('/')
const entries = ref<FileEntry[]>([])
const loading = ref(false)
const pathSegments = computed(() => currentPath.value.split('/').filter(Boolean))

const fileColumns = [
  { colKey: 'name', title: '名称', minWidth: 240 },
  { colKey: 'size', title: '大小', width: 100 },
  { colKey: 'mode', title: '权限', width: 130 },
  { colKey: 'mod_time', title: '修改时间', minWidth: 160 },
  { colKey: 'operations', title: '操作', width: 280, fixed: 'right' as const },
]

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

async function navigateTo(path: string) { currentPath.value = path || '/'; await reload() }

async function onRowDblClick({ row }: { row: FileEntry }) {
  if (!row.is_dir) return
  await navigateTo(fullPath(row.name))
}

async function reload() {
  loading.value = true
  try {
    const res = await listFiles(serverId.value, currentPath.value)
    entries.value = res.entries ?? []
  } catch { MessagePlugin.error('读取目录失败') }
  finally { loading.value = false }
}

const uploadInput = ref<HTMLInputElement>()
function triggerUpload() { uploadInput.value?.click() }
async function onUploadChange(e: Event) {
  const files = (e.target as HTMLInputElement).files
  if (!files) return
  for (const file of Array.from(files)) {
    try { await uploadFile(serverId.value, currentPath.value, file); MessagePlugin.success(`${file.name} 上传成功`) }
    catch { MessagePlugin.error(`${file.name} 上传失败`) }
  }
  ;(e.target as HTMLInputElement).value = ''
  await reload()
}

async function downloadEntry(row: FileEntry) {
  try { await downloadFile(serverId.value, fullPath(row.name), auth.token) }
  catch { MessagePlugin.error('下载失败') }
}

async function deleteEntry(row: FileEntry) {
  try { await deleteFile(serverId.value, fullPath(row.name)); MessagePlugin.success('已删除'); await reload() }
  catch { MessagePlugin.error('删除失败') }
}

const mkdirVisible = ref(false)
const mkdirName = ref('')
function openMkdir() { mkdirVisible.value = true }
async function confirmMkdir() {
  if (!mkdirName.value.trim()) return
  try { await mkdir(serverId.value, fullPath(mkdirName.value.trim())); MessagePlugin.success('目录已创建'); mkdirVisible.value = false; await reload() }
  catch { MessagePlugin.error('创建失败') }
}

const renameVisible = ref(false)
const renameFrom = ref('')
const renameTo = ref('')
function openRename(row: FileEntry) { renameFrom.value = fullPath(row.name); renameTo.value = renameFrom.value; renameVisible.value = true }
async function confirmRename() {
  try { await renameFile(serverId.value, renameFrom.value, renameTo.value); MessagePlugin.success('重命名成功'); renameVisible.value = false; await reload() }
  catch { MessagePlugin.error('重命名失败') }
}

const chmodVisible = ref(false)
const chmodPath = ref('')
const chmodMode = ref('')
function openChmod(row: FileEntry) { chmodPath.value = fullPath(row.name); chmodMode.value = ''; chmodVisible.value = true }
async function confirmChmod() {
  if (!chmodMode.value.trim()) return
  try { await chmod(serverId.value, chmodPath.value, chmodMode.value.trim()); MessagePlugin.success('权限已修改'); chmodVisible.value = false; await reload() }
  catch { MessagePlugin.error('修改权限失败') }
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
  } catch (e: any) { MessagePlugin.error(e?.response?.data?.msg ?? '读取文件失败'); editVisible.value = false }
}

async function saveEdit() {
  if (!editorView) return
  saving.value = true
  try {
    await putFileContent(serverId.value, editPath.value, editorView.state.doc.toString())
    MessagePlugin.success('保存成功'); editVisible.value = false
  } catch { MessagePlugin.error('保存失败') }
  finally { saving.value = false }
}

function destroyEditor() { editorView?.destroy(); editorView = null }

onMounted(() => reload())
onBeforeUnmount(() => editorView?.destroy())
</script>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
  flex-wrap: wrap;
  gap: 8px;
}

.path-breadcrumb {
  flex: 1;
  min-width: 200px;
  cursor: pointer;
}

.toolbar-right {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}

.file-name-cell {
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  user-select: none;
}

.file-icon {
  flex-shrink: 0;
  font-size: 16px;
}

.file-icon.dir { color: var(--sh-blue); }
.file-icon.file { color: #8a94a6; }

.mono-text {
  font-family: "Cascadia Code", "JetBrains Mono", Menlo, monospace;
  font-size: 12px;
}

.perm {
  color: var(--sh-text-secondary);
}

.code-editor {
  height: 60vh;
  overflow: auto;
  border: 1px solid var(--sh-border);
  border-radius: 4px;
  font-size: 13px;
}

:deep(.cm-editor) { height: 100%; }
:deep(.cm-scroller) { overflow: auto; }

:deep(.t-table) { font-size: 13px; }
</style>
