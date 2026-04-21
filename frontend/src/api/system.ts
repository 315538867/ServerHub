import request from './request'

export interface FirewallRule {
  index: number
  rule: string
  action: string
  from: string
  type: string
}

export interface CronJob {
  index: number
  expr: string
  cmd: string
  raw: string
}

export interface ProcessItem {
  user: string
  pid: string
  cpu: number
  mem: number
  command: string
}

export interface ServiceItem {
  unit: string
  load: string
  active: string
  sub: string
  description: string
}

export function getFirewallRules(sid: number) {
  return request.get<never, { type: string; rules: FirewallRule[] }>(`/servers/${sid}/system/firewall/rules`)
}
export function addFirewallRule(sid: number, data: { port: string; proto: string; action: string; from?: string }) {
  return request.post(`/servers/${sid}/system/firewall/rules`, data)
}
export function deleteFirewallRule(sid: number, rule: string) {
  return request.delete(`/servers/${sid}/system/firewall/rules`, { params: { rule } })
}

export function getCronJobs(sid: number) {
  return request.get<never, CronJob[]>(`/servers/${sid}/system/cron/jobs`)
}
export function addCronJob(sid: number, expr: string, cmd: string) {
  return request.post(`/servers/${sid}/system/cron/jobs`, { expr, cmd })
}
export function updateCronJob(sid: number, index: number, expr: string, cmd: string) {
  return request.put(`/servers/${sid}/system/cron/jobs`, { index, expr, cmd })
}
export function deleteCronJob(sid: number, index: number) {
  return request.delete(`/servers/${sid}/system/cron/jobs`, { params: { index } })
}

export function getProcesses(sid: number) {
  return request.get<never, ProcessItem[]>(`/servers/${sid}/system/processes`)
}
export function killProcess(sid: number, pid: string) {
  return request.delete(`/servers/${sid}/system/processes/${pid}`)
}

export function getServices(sid: number) {
  return request.get<never, ServiceItem[]>(`/servers/${sid}/system/services`)
}
export function serviceAction(sid: number, name: string, action: string) {
  return request.post(`/servers/${sid}/system/services/${encodeURIComponent(name)}/action`, { action })
}
export function serviceLogsWsUrl(sid: number, name: string, token: string) {
  return `ws://${location.host}/panel/api/v1/servers/${sid}/system/services/${encodeURIComponent(name)}/logs?token=${token}`
}

export interface SelfMetrics {
  cpu_percent: number
  mem_rss: number
  mem_sys: number
  goroutines: number
  uptime: number
  connections: number
  num_cpu: number
  history: { cpu: number[]; mem: number[] }
}

export function getSelfMetrics() {
  return request.get<never, SelfMetrics>('/system/self')
}
