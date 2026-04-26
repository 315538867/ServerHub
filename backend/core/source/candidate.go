package source

// Candidate 是 Scanner 在远端 server 上发现的候选服务。
//
// 候选物化为 domain.Service 之前由 usecase 校验、用户确认。
type Candidate struct {
	Kind        string            // = Scanner.Kind()
	Name        string            // 候选名(用户可改)
	Image       string            // docker 才有
	Cmd         string            // native/systemd 才有
	ConfigFiles []string          // 远端绝对路径
	Suggested   SuggestedFields   // 建议填充到 Service 的字段
	Raw         map[string]string // adapter 自定义元数据
}

// SuggestedFields 是 Scanner 推断出的可填字段。
// usecase 在物化时把这些值合并到 Service。
type SuggestedFields struct {
	EnvVars map[string]string
	Ports   []string
	Volumes []string
	Workdir string
}
