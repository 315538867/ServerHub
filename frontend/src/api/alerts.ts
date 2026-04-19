import request from './request'

export interface AlertRule {
  id: number
  server_id: number
  metric: string
  operator: string
  threshold: number
  duration: number
  enabled: boolean
  created_at: string
}

export interface AlertEvent {
  id: number
  rule_id: number
  server_id: number
  value: number
  message: string
  sent_at: string
}

export interface NotifyChannel {
  id: number
  name: string
  type: string
  template: string
  enabled: boolean
  created_at: string
}

export function listRules(): Promise<AlertRule[]> {
  return request.get('/alerts/rules')
}

export function createRule(body: { server_id?: number; metric: string; operator?: string; threshold: number; duration?: number }) {
  return request.post('/alerts/rules', body)
}

export function updateRule(id: number, body: Partial<{ operator: string; threshold: number; duration: number; enabled: boolean }>) {
  return request.put(`/alerts/rules/${id}`, body)
}

export function deleteRule(id: number) {
  return request.delete(`/alerts/rules/${id}`)
}

export function listEvents(page = 1, size = 50): Promise<{ total: number; events: AlertEvent[] }> {
  return request.get('/alerts/events', { params: { page, size } })
}

export function clearEvents() {
  return request.delete('/alerts/events')
}

export function listChannels(): Promise<NotifyChannel[]> {
  return request.get('/alerts/channels')
}

export function createChannel(body: { name: string; type: string; url: string; template?: string }) {
  return request.post('/alerts/channels', body)
}

export function updateChannel(id: number, body: Partial<{ name: string; url: string; template: string; enabled: boolean }>) {
  return request.put(`/alerts/channels/${id}`, body)
}

export function deleteChannel(id: number) {
  return request.delete(`/alerts/channels/${id}`)
}

export function testChannel(id: number) {
  return request.post(`/alerts/channels/${id}/test`)
}
