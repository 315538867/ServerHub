package runtime

import "fmt"

// RemoteError 表示远端命令执行失败,usecase 层应包装为 5xx。
type RemoteError struct {
	Op  string // 操作名(如 "docker inspect")
	Err error  // 原始错误
}

func (e *RemoteError) Error() string {
	return fmt.Sprintf("runtime remote error: %s: %v", e.Op, e.Err)
}

func (e *RemoteError) Unwrap() error { return e.Err }

// ConfigError 表示 StartSpec/Service 配置不合法,usecase 层应包装为 422。
type ConfigError struct {
	Field  string
	Reason string
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("runtime config error: %s: %s", e.Field, e.Reason)
}
