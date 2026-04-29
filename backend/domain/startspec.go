package domain

import (
	"encoding/json"
	"fmt"
)

// StartSpec 是 Release 启动配置的类型化接口。持久化为 JSON string 存储在 model 层，
// 内存中使用此接口及实现 struct。
type StartSpec interface {
	Kind() string
	Validate() error
}

// DockerSpec 是 docker 单容器启动配置。
type DockerSpec struct {
	Image string `json:"image"`
	Cmd   string `json:"cmd,omitempty"`
}

func (DockerSpec) Kind() string  { return string(ServiceTypeDocker) }
func (s DockerSpec) Validate() error {
	if s.Image == "" {
		return fmt.Errorf("DockerSpec: image required")
	}
	return nil
}

// ComposeSpec 是 docker-compose 启动配置。
type ComposeSpec struct {
	FileName string `json:"file_name"`
}

func (ComposeSpec) Kind() string  { return string(ServiceTypeCompose) }
func (s ComposeSpec) Validate() error {
	if s.FileName == "" {
		return fmt.Errorf("ComposeSpec: file_name required")
	}
	return nil
}

// NativeSpec 是裸进程启动配置。
type NativeSpec struct {
	Cmd string `json:"cmd"`
}

func (NativeSpec) Kind() string  { return string(ServiceTypeNative) }
func (s NativeSpec) Validate() error {
	if s.Cmd == "" {
		return fmt.Errorf("NativeSpec: cmd required")
	}
	return nil
}

// StaticSpec 是纯静态资源启动配置（无独立进程）。
type StaticSpec struct{}

func (StaticSpec) Kind() string  { return string(ServiceTypeStatic) }
func (StaticSpec) Validate() error { return nil }

// UnmarshalStartSpec 从 JSON 字符串反序列化为 typed StartSpec。
// kind 为空时按 JSON shape 自动推断(image → docker, file_name → compose, cmd → native, 否则 static)。
func UnmarshalStartSpec(raw string) (StartSpec, error) {
	if raw == "" || raw == "{}" {
		return &StaticSpec{}, nil
	}

	// 先探查 shape 推断 kind
	var probe struct {
		Image    string `json:"image"`
		FileName string `json:"file_name"`
		Cmd      string `json:"cmd"`
	}
	if err := json.Unmarshal([]byte(raw), &probe); err != nil {
		return nil, fmt.Errorf("UnmarshalStartSpec: %w", err)
	}

	switch {
	case probe.Image != "":
		var s DockerSpec
		if err := json.Unmarshal([]byte(raw), &s); err != nil {
			return nil, fmt.Errorf("DockerSpec: %w", err)
		}
		return &s, nil
	case probe.FileName != "":
		var s ComposeSpec
		if err := json.Unmarshal([]byte(raw), &s); err != nil {
			return nil, fmt.Errorf("ComposeSpec: %w", err)
		}
		return &s, nil
	case probe.Cmd != "":
		var s NativeSpec
		if err := json.Unmarshal([]byte(raw), &s); err != nil {
			return nil, fmt.Errorf("NativeSpec: %w", err)
		}
		return &s, nil
	default:
		return &StaticSpec{}, nil
	}
}

// UnmarshalStartSpecByKind 按明确的 Service.Type(kind) 反序列化，不做推断。
// prefer 用于已知确切类型时（如 API handler 已有 service.Type）。
func UnmarshalStartSpecByKind(kind, raw string) (StartSpec, error) {
	if raw == "" {
		raw = "{}"
	}
	switch kind {
	case string(ServiceTypeDocker):
		var s DockerSpec
		if err := json.Unmarshal([]byte(raw), &s); err != nil {
			return nil, fmt.Errorf("DockerSpec: %w", err)
		}
		return &s, nil
	case string(ServiceTypeCompose):
		var s ComposeSpec
		if err := json.Unmarshal([]byte(raw), &s); err != nil {
			return nil, fmt.Errorf("ComposeSpec: %w", err)
		}
		return &s, nil
	case string(ServiceTypeNative):
		var s NativeSpec
		if err := json.Unmarshal([]byte(raw), &s); err != nil {
			return nil, fmt.Errorf("NativeSpec: %w", err)
		}
		return &s, nil
	case string(ServiceTypeStatic):
		return &StaticSpec{}, nil
	default:
		return nil, fmt.Errorf("UnmarshalStartSpecByKind: unknown kind %q", kind)
	}
}
