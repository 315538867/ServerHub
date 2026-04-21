import request from './request'

export interface SuggestedDeploy {
  type: string
  work_dir: string
  compose_file?: string
  start_cmd?: string
  image_name?: string
  runtime?: string
}

export interface Candidate {
  kind: 'docker' | 'compose' | 'systemd'
  source_id: string
  name: string
  summary: string
  suggested: SuggestedDeploy
}

export interface ScanResult {
  docker: Candidate[]
  compose: Candidate[]
  systemd: Candidate[]
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
  payload: { docker: Candidate[]; compose: Candidate[]; systemd: Candidate[] },
) {
  return request.post<ImportResult>(`/servers/${id}/discover/import`, payload)
}
