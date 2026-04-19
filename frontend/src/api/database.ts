import request from './request'

export interface DBConn {
  id: number
  server_id: number
  name: string
  type: 'mysql' | 'redis'
  host: string
  port: number
  username: string
  database: string
}

export interface CreateDBConnBody {
  server_id: number
  name: string
  type: 'mysql' | 'redis'
  host?: string
  port?: number
  username?: string
  password?: string
  database?: string
}

export function listConns(serverId?: number) {
  const params = serverId ? { server_id: serverId } : {}
  return request.get<DBConn[]>('/dbconns', { params })
}

export function createConn(body: CreateDBConnBody) {
  return request.post('/dbconns', body)
}

export function updateConn(id: number, body: Partial<CreateDBConnBody & { password: string }>) {
  return request.put(`/dbconns/${id}`, body)
}

export function deleteConn(id: number) {
  return request.delete(`/dbconns/${id}`)
}

export function testConn(id: number) {
  return request.post(`/dbconns/${id}/test`)
}

// MySQL
export function mysqlDatabases(id: number): Promise<string[]> {
  return request.get(`/dbconns/${id}/mysql/databases`)
}

export function mysqlCreateDatabase(id: number, name: string, charset?: string) {
  return request.post(`/dbconns/${id}/mysql/databases`, { name, charset })
}

export function mysqlDropDatabase(id: number, dbname: string) {
  return request.delete(`/dbconns/${id}/mysql/databases/${dbname}`)
}

export function mysqlUsers(id: number) {
  return request.get<Array<{ user: string; host: string }>>(`/dbconns/${id}/mysql/users`)
}

export function mysqlCreateUser(id: number, body: { user: string; host?: string; password: string; database?: string; grant?: string }) {
  return request.post(`/dbconns/${id}/mysql/users`, body)
}

export interface QueryResult {
  columns: string[]
  rows: string[][]
}

export function mysqlQuery(id: number, sql: string, database?: string): Promise<QueryResult> {
  return request.post(`/dbconns/${id}/mysql/query`, { sql, database })
}

export function mysqlStatus(id: number): Promise<QueryResult> {
  return request.get(`/dbconns/${id}/mysql/status`)
}

export function mysqlExportUrl(id: number, dbname: string, token: string) {
  return `/panel/api/v1/dbconns/${id}/mysql/export/${dbname}?token=${token}`
}

// Redis
export function redisInfo(id: number): Promise<Record<string, string>> {
  return request.get(`/dbconns/${id}/redis/info`)
}

export function redisKeys(id: number, pattern?: string): Promise<string[]> {
  return request.get(`/dbconns/${id}/redis/keys`, { params: { pattern } })
}

export function redisGetKey(id: number, key: string) {
  return request.get(`/dbconns/${id}/redis/keys/${encodeURIComponent(key)}`)
}

export function redisDelKey(id: number, key: string) {
  return request.delete(`/dbconns/${id}/redis/keys/${encodeURIComponent(key)}`)
}

export function redisFlushDB(id: number) {
  return request.post(`/dbconns/${id}/redis/flushdb`, { confirm: 'FLUSHDB' })
}
