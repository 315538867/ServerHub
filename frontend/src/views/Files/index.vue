<template>
  <div class="files-page">
    <!-- Toolbar -->
    <div class="page-toolbar">
      <el-select v-model="selectedServerId" placeholder="选择服务器" style="width:220px" @change="navigateTo('/')">
        <el-option v-for="s in servers" :key="s.id" :label="`${s.name} (${s.host})`" :value="s.id" />
      </el-select>

      <el-breadcrumb separator="/" class="path-breadcrumb">
        <el-breadcrumb-item style="cursor:pointer" @click="navigateTo('/')">根目录</el-breadcrumb-item>
        <el-breadcrumb-item
          v-for="(seg, i) in pathSegments"
          :key="i"
          style="cursor:pointer"
          @click="navigateTo('/' + pathSegments.slice(0, i + 1).join('/'))"
        >{{ seg }}</el-breadcrumb-item>
      </el-breadcrumb>

      <div class="toolbar-right">
        <el-button :icon="Upload" @click="triggerUpload">上传文件</el-button>
        <el-button :icon="FolderAdd" @click="openMkdir">新建目录</el-button>
        <el-button :icon="Refresh" :loading="loading" @click="reload">刷新</el-button>
      </div>
    </div>

    <!-- File table -->
    <el-table
      :data="entries"
      v-loading="loading"
      style="width:100%"
      empty-text="目录为空"
      @row-dblclick="onRowDblClick"
    >
      <el-table-column label="名称" min-width="240">
        <template #default="{ row }">
          <div class="file-name-cell">
            <el-icon :color="row.is_dir ? '#409eff' : '#909399'">
              <Folder v-if="row.is_dir" />
              <Document v-else />
            </el-icon>
            <span>{{ row.name }}</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="大小" width="100">
        <template #default="{ row }">{{ row.is_dir ? '—' : formatSize(row.size) }}</template>
      </el-table-column>
      <el-table-column label="权限" prop="mode" width="130" />
      <el-table-column label="修改时间" prop="mod_time" min-width="160" />
      <el-table-column label="操作" width="260" fixed="right">
        <template #default="{ row }">
          <el-button v-if="!row.is_dir" size="small" @click="downloadEntry(row)">下载</el-button>
          <el-button v-if="!row.is_dir && isEditable(row.name)" size="small" @click="openEdit(row)">编辑</el-button>
          <el-button size="small" @click="openRename(row)">重命名</el-button>
          <el-button size="small" @click="openChmod(row)">权限</el-button>
          <el-popconfirm :title="`确认删除 ${row.name}？`" @confirm="deleteEntry(row)">
            <template #reference>
              <el-button size="small" type="danger">删除</el-button>
            </template>
          </el-popconfirm>
        </template>
      </el-table-column>
    </el-table>

    <!-- Hidden upload input -->
    <input ref="uploadInput" type="file" multiple style="display:none" @change="onUploadChange" />

    <!-- Mkdir dialog -->
    <el-dialog v-model="mkdirVisible" title="新建目录" width="400px" @closed="mkdirName = ''">
      <el-input v-model="mkdirName" placeholder="目录名" @keyup.enter="confirmMkdir" />
      <template #footer>
        <el-button @click="mkdirVisible = false">取消</el-button>
        <el-button type="primary" @click="confirmMkdir">创建</el-button>
      </template>
    </el-dialog>

    <!-- Rename dialog -->
    <el-dialog v-model="renameVisible" title="重命名 / 移动" width="400px">
      <el-input v-model="renameTo" placeholder="新路径" />
      <template #footer>
        <el-button @click="renameVisible = false">取消</el-button>
        <el-button type="primary" @click="confirmRename">确认</el-button>
      </template>
    </el-dialog>

    <!-- Chmod dialog -->
    <el-dialog v-model="chmodVisible" title="修改权限" width="400px">
      <el-input v-model="chmodMode" placeholder="如 0644 或 755" />
      <template #footer>
        <el-button @click="chmodVisible = false">取消</el-button>
        <el-button type="primary" @click="confirmChmod">确认</el-button>
      </template>
    </el-dialog>

    <!-- Edit dialog -->
    <el-dialog
      v-model="editVisible"
      :title="`编辑 — ${editPath}`"
      width="800px"
      top="4vh"
      :close-on-click-modal="false"
      @closed="destroyEditor"
    >
      <div ref="editorEl" class="code-editor" />
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveEdit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onBeforeUnmount } from 'vue'
import { Refresh, Upload, FolderAdd, Folder, Document } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { EditorView, basicSetup } from 'codemirror'
import { EditorState } from '@codemirror/state'
import { json } from '@codemirror/lang-json'
import { yaml } from '@codemirror/lang-yaml'
import { javascript } from '@codemirror/lang-javascript'
import { oneDark } from '@codemirror/theme-one-dark'
import { useAuthStore } from '@/stores/auth'
import { getServers } from '@/api/servers'
import {
  listFiles, getFileContent, putFileContent, downloadFile,
  uploadFile, mkdir, deleteFile, renameFile, chmod,
} from '@/api/files'
import type { FileEntry } from '@/api/files'
import type { Server } from '@/types/api'

const auth = useAuthStore()
const servers = ref<Server[]>([])
const selectedServerId = ref<number | null>(null)
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

// ── Navigation ────────────────────────────────────────────────────
async function navigateTo(path: string) {
  currentPath.value = path || '/'
  await reload()
}

async function onRowDblClick(row: FileEntry) {
  if (!row.is_dir) return
  await navigateTo(fullPath(row.name))
}

async function reload() {
  if (!selectedServerId.value) return
  loading.value = true
  try {
    const res = await listFiles(selectedServerId.value, currentPath.value)
    entries.value = res.entries ?? []
  } catch {
    ElMessage.error('读取目录失败')
  } finally {
    loading.value = false
  }
}

// ── Upload ────────────────────────────────────────────────────────
const uploadInput = ref<HTMLInputElement>()

function triggerUpload() {
  if (!selectedServerId.value) { ElMessage.warning('请先选择服务器'); return }
  uploadInput.value?.click()
}

async function onUploadChange(e: Event) {
  const files = (e.target as HTMLInputElement).files
  if (!files || !selectedServerId.value) return
  for (const file of Array.from(files)) {
    try {
      await uploadFile(selectedServerId.value, currentPath.value, file)
      ElMessage.success(`${file.name} 上传成功`)
    } catch {
      ElMessage.error(`${file.name} 上传失败`)
    }
  }
  ;(e.target as HTMLInputElement).value = ''
  await reload()
}

// ── Download ─────────────────────────────────────────────────────
async function downloadEntry(row: FileEntry) {
  if (!selectedServerId.value) return
  try {
    await downloadFile(selectedServerId.value, fullPath(row.name), auth.token)
  } catch {
    ElMessage.error('下载失败')
  }
}

// ── Delete ───────────────────────────────────────────────────────
async function deleteEntry(row: FileEntry) {
  if (!selectedServerId.value) return
  try {
    await deleteFile(selectedServerId.value, fullPath(row.name))
    ElMessage.success('已删除')
    await reload()
  } catch {
    ElMessage.error('删除失败')
  }
}

// ── Mkdir ─────────────────────────────────────────────────────────
const mkdirVisible = ref(false)
const mkdirName = ref('')

function openMkdir() {
  if (!selectedServerId.value) { ElMessage.warning('请先选择服务器'); return }
  mkdirVisible.value = true
}

async function confirmMkdir() {
  if (!mkdirName.value.trim() || !selectedServerId.value) return
  try {
    await mkdir(selectedServerId.value, fullPath(mkdirName.value.trim()))
    ElMessage.success('目录已创建')
    mkdirVisible.value = false
    await reload()
  } catch {
    ElMessage.error('创建失败')
  }
}

// ── Rename ────────────────────────────────────────────────────────
const renameVisible = ref(false)
const renameFrom = ref('')
const renameTo = ref('')

function openRename(row: FileEntry) {
  renameFrom.value = fullPath(row.name)
  renameTo.value = renameFrom.value
  renameVisible.value = true
}

async function confirmRename() {
  if (!selectedServerId.value) return
  try {
    await renameFile(selectedServerId.value, renameFrom.value, renameTo.value)
    ElMessage.success('重命名成功')
    renameVisible.value = false
    await reload()
  } catch {
    ElMessage.error('重命名失败')
  }
}

// ── Chmod ─────────────────────────────────────────────────────────
const chmodVisible = ref(false)
const chmodPath = ref('')
const chmodMode = ref('')

function openChmod(row: FileEntry) {
  chmodPath.value = fullPath(row.name)
  chmodMode.value = ''
  chmodVisible.value = true
}

async function confirmChmod() {
  if (!selectedServerId.value || !chmodMode.value.trim()) return
  try {
    await chmod(selectedServerId.value, chmodPath.value, chmodMode.value.trim())
    ElMessage.success('权限已修改')
    chmodVisible.value = false
    await reload()
  } catch {
    ElMessage.error('修改权限失败')
  }
}

// ── Edit ─────────────────────────────────────────────────────────
const editVisible = ref(false)
const editPath = ref('')
const saving = ref(false)
const editorEl = ref<HTMLDivElement>()
let editorView: EditorView | null = null

async function openEdit(row: FileEntry) {
  if (!selectedServerId.value) return
  editPath.value = fullPath(row.name)
  editVisible.value = true
  await nextTick()
  try {
    const res = await getFileContent(selectedServerId.value, editPath.value)
    editorView?.destroy()
    editorView = new EditorView({
      state: EditorState.create({
        doc: res.content,
        extensions: [basicSetup, oneDark, ...getLang(row.name)],
      }),
      parent: editorEl.value!,
    })
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.msg ?? '读取文件失败')
    editVisible.value = false
  }
}

async function saveEdit() {
  if (!selectedServerId.value || !editorView) return
  saving.value = true
  try {
    await putFileContent(selectedServerId.value, editPath.value, editorView.state.doc.toString())
    ElMessage.success('保存成功')
    editVisible.value = false
  } catch {
    ElMessage.error('保存失败')
  } finally {
    saving.value = false
  }
}

function destroyEditor() {
  editorView?.destroy()
  editorView = null
}

onBeforeUnmount(() => editorView?.destroy())

async function init() {
  servers.value = await getServers()
  if (servers.value.length) {
    selectedServerId.value = servers.value[0].id
    await reload()
  }
}

init()
</script>

<style scoped>
.files-page { padding: 20px; }

.page-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.path-breadcrumb { flex: 1; min-width: 200px; }
.toolbar-right { display: flex; gap: 8px; }

.file-name-cell {
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  user-select: none;
}

.code-editor {
  height: 60vh;
  overflow: auto;
  border: 1px solid #e4e7ed;
  border-radius: 4px;
  font-size: 13px;
}

:deep(.cm-editor) { height: 100%; }
:deep(.cm-scroller) { overflow: auto; }
</style>
