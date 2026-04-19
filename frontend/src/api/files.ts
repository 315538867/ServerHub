import request from './request'

export interface FileEntry {
  name: string
  is_dir: boolean
  size: number
  mode: string
  mod_time: string
}

export interface ListResult {
  path: string
  entries: FileEntry[]
}

export function listFiles(serverId: number, path = '/') {
  return request.get<never, ListResult>(`/servers/${serverId}/files/list`, { params: { path } })
}

export function getFileContent(serverId: number, path: string) {
  return request.get<never, { path: string; content: string }>(`/servers/${serverId}/files/content`, { params: { path } })
}

export function putFileContent(serverId: number, path: string, content: string) {
  return request.put(`/servers/${serverId}/files/content`, { path, content })
}

export async function downloadFile(serverId: number, filePath: string, token: string) {
  const res = await fetch(`/panel/api/v1/servers/${serverId}/files/download?path=${encodeURIComponent(filePath)}`, {
    headers: { Authorization: `Bearer ${token}` },
  })
  if (!res.ok) throw new Error('download failed')
  const blob = await res.blob()
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filePath.split('/').pop() ?? 'file'
  a.click()
  URL.revokeObjectURL(url)
}

export function uploadFile(serverId: number, dir: string, file: File, onProgress?: (pct: number) => void) {
  const form = new FormData()
  form.append('file', file)
  form.append('path', dir)
  return request.post(`/servers/${serverId}/files/upload`, form, {
    onUploadProgress: (e) => {
      if (onProgress && e.total) onProgress(Math.round((e.loaded * 100) / e.total))
    },
  })
}

export function mkdir(serverId: number, path: string) {
  return request.post(`/servers/${serverId}/files/mkdir`, { path })
}

export function deleteFile(serverId: number, path: string) {
  return request.delete(`/servers/${serverId}/files/delete`, { params: { path } })
}

export function renameFile(serverId: number, oldPath: string, newPath: string) {
  return request.post(`/servers/${serverId}/files/rename`, { old_path: oldPath, new_path: newPath })
}

export function chmod(serverId: number, path: string, mode: string) {
  return request.post(`/servers/${serverId}/files/chmod`, { path, mode })
}
