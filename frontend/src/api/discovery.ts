import request from './request'

export interface EnvKV {
  key: string
  value: string
  secret?: boolean
}

export interface SuggestedDeploy {
  type: string
  work_dir: string
  compose_file?: string
  start_cmd?: string
  image_name?: string
  runtime?: string
  env?: EnvKV[]
}

export interface Candidate {
  kind: 'docker' | 'compose' | 'systemd' | 'nginx'
  source_id: string
  name: string
  summary: string
  suggested: SuggestedDeploy
  extra_labels?: Record<string, string>
  fingerprint?: string
  already_managed?: boolean
}

export interface ScanResult {
  docker: Candidate[]
  compose: Candidate[]
  systemd: Candidate[]
  nginx: Candidate[]
  errors?: string[]
}

export interface ImportResult {
  imported: number
  skipped: number
  errors?: string[]
}

export function scanServer(id: number, kinds?: string[]) {
  const q = kinds && kinds.length ? `?kinds=${kinds.join(',')}` : ''
  return request.get<ScanResult>(`/servers/${id}/discover${q}`)
}

export function importCandidates(
  id: number,
  payload: { docker: Candidate[]; compose: Candidate[]; systemd: Candidate[]; nginx: Candidate[] },
) {
  return request.post<ImportResult>(`/servers/${id}/discover/import`, payload)
}

export interface TakeoverResult {
  deploy_id?: number
  success: boolean
  rolled_back: boolean
  output: string
  error?: string
}

export function takeoverCandidate(
  serverId: number,
  payload: { candidate: Candidate; target_name: string },
) {
  return request.post<TakeoverResult>(`/servers/${serverId}/discover/takeover`, payload)
}
