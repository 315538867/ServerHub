package source

// Candidate 是 Scanner 在远端 server 上发现的候选服务。
//
// 候选物化为 domain.Service 之前由 usecase 校验、用户确认。
//
// R4 起字段补齐(配合 pkg/discovery 平移到 adapters/source/<kind>):
//   - SourceID 是远端稳定标识符,与 (server_id, kind, source_id) 一起作为接管幂等键;
//     例: docker container.ID、systemd unit name、nginx site config path
//   - Summary 是给前端的一行人类描述,展示在发现列表
//   - AlreadyManaged 由 usecase 后置回填,Scanner 不写
//   - Suggested 字段补齐部署规格,接管时可直接物化到 model.Service
//   - Raw 是 adapter 自定义元数据,Fingerprint 算法的输入(如 docker 的 binds/ports
//     排序串、systemd 的 exec_start),Scanner 自行约定 key
type Candidate struct {
	Kind           string            // = Scanner.Kind()
	SourceID       string            // 远端稳定标识符,接管幂等键的一部分
	Name           string            // 候选名(用户可改)
	Summary        string            // 一行人类描述
	Image          string            // docker 才有
	Cmd            string            // native/systemd 才有
	ConfigFiles    []string          // 远端绝对路径
	Suggested      SuggestedFields   // 建议填充到 Service 的字段
	Raw            map[string]string // adapter 自定义元数据(指纹输入)
	AlreadyManaged bool              // usecase 回填:同 server 已有 fingerprint 命中
}

// SuggestedFields 是 Scanner 推断出的可填字段。
// usecase 在物化时把这些值合并到 Service。
//
// R4 起字段对齐 v1 SuggestedDeploy 完整集:
//   - Type/Runtime/StartCmd/Image/ComposeFile 是 Service 部署规格
//   - EnvSecrets[k]=true 表示该 env key 的 value 是敏感信息,
//     usecase 物化时压到 model.EnvVarSet 的 AES-GCM 加密字段(替代 v1 EnvKV.Secret 位)
type SuggestedFields struct {
	Type        string          // deploy 类型:docker / compose / static / systemd
	Runtime     string          // 容器运行时:docker / podman
	StartCmd    string          // native/systemd 启动命令
	Image       string          // 镜像 ref(docker/compose)
	ComposeFile string          // compose.yaml 远端绝对路径
	Workdir     string
	EnvVars     map[string]string
	EnvSecrets  map[string]bool // EnvVars[k] 是否敏感
	Ports       []string
	Volumes     []string
}
