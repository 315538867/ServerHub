import request from './request'

export interface ContainerItem {
  id: string
  names: string
  image: string
  status: string
  state: string
  ports: string
  created_at: string
}

export interface ImageItem {
  id: string
  repository: string
  tag: string
  size: string
  created_at: string
}

export function getContainers(serverId: number) {
  return request.get<never, ContainerItem[]>(`/servers/${serverId}/docker/containers`)
}

export function containerAction(serverId: number, cid: string, action: 'start' | 'stop' | 'restart' | 'remove') {
  return request.post<never, { output: string }>(`/servers/${serverId}/docker/containers/${cid}/action`, { action })
}

export function getContainerInspect(serverId: number, cid: string) {
  return request.get<never, any[]>(`/servers/${serverId}/docker/containers/${cid}/inspect`)
}

export function getImages(serverId: number) {
  return request.get<never, ImageItem[]>(`/servers/${serverId}/docker/images`)
}

export function deleteImage(serverId: number, iid: string) {
  return request.delete<never, { output: string }>(`/servers/${serverId}/docker/images/${iid}`)
}

export function containerLogsWsUrl(serverId: number, cid: string, token: string) {
  return `ws://${location.host}/panel/api/v1/servers/${serverId}/docker/containers/${cid}/logs?token=${token}`
}

export function pullImageWsUrl(serverId: number, image: string, token: string) {
  return `ws://${location.host}/panel/api/v1/servers/${serverId}/docker/images/pull?image=${encodeURIComponent(image)}&token=${token}`
}
