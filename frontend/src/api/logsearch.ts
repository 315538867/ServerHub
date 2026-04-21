import request from './request'

export interface LogSearchReq {
  source: 'docker' | 'journalctl' | 'nginx-access' | 'nginx-error'
  target?: string
  query: string
  regex?: boolean
  case_sensitive?: boolean
  since?: string
  context?: number
  limit?: number
}

export interface LogSearchLine {
  raw: string
}

export interface LogSearchResp {
  lines: LogSearchLine[]
  truncated: boolean
  error?: string
}

export function searchLogs(sid: number, body: LogSearchReq) {
  return request.post<never, LogSearchResp>(`/servers/${sid}/logs/search`, body)
}
