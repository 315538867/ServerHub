// Package nginxops 实现 Nginx 编排的运行期能力：
//
//   - Snapshot/Restore：对 /etc/nginx 做 tar 级备份，作为 reload 失败时的
//     人工 breakglass 还原入口；
//   - Differ：对比 desired ConfigFile 与 actual 远端文件，输出 add/update/delete；
//   - Lock：按 edge_server_id 互斥，防止并发 apply 写同一台 edge；
//   - Reconciler：把上述三件套与 nginxrender + netresolve 串联成原子幂等的
//     Apply 与 DryRun 流程；并写入 audit_apply 与 ingress.status。
//
// 本包是 P1 编排闭环的运行期核心，所有 nginx 远端操作必须通过 runner.Runner，
// 写文件必须经 safeshell.WriteRemoteFile，与跨期约束保持一致。
package nginxops

// ChangeKind 标识 Differ 输出的一条变更类别。
type ChangeKind string

const (
	ChangeAdd    ChangeKind = "add"
	ChangeUpdate ChangeKind = "update"
	ChangeDelete ChangeKind = "delete"
)

// Change 是 Differ 输出的最小单元。
//   - add:    远端缺该 path，NewContent/NewHash 有效，OldContent/OldHash 空
//   - update: 远端存在但 sha256 不同，新旧都有效；回滚时用 OldContent 写回
//   - delete: 远端存在且为我们管的文件，但 desired 中无；OldContent/OldHash 有效，
//             NewContent 空。回滚时把 OldContent 写回。
type Change struct {
	Kind       ChangeKind
	Path       string
	NewContent string
	NewHash    string
	OldContent string
	OldHash    string
}

// ApplyResult 是 Reconciler.Apply 的返回值。
type ApplyResult struct {
	AuditID    uint     // audit_apply 行 ID
	Changes    []Change // 实际落盘的变更列表
	NoOp       bool     // true 表示无差异，未触碰文件系统
	Output     string   // nginx -t / nginx -s reload 的合并输出
	BackupPath string   // 本次 apply 的 tar 备份路径
	RolledBack bool     // 失败回滚标记
}
