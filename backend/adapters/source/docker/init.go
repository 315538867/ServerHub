// Package docker 是 SourceScanner adapter,负责发现并接管独立 docker 容器
// (有 com.docker.compose.project label 的归 adapters/source/compose)。
//
// 平移自 v1 pkg/discovery/docker.go + pkg/takeover/docker.go,签名改用
// core/source.Scanner + infra.Runner + internal/stepkit。
package docker

import "github.com/serverhub/serverhub/core/source"

func init() { source.Default.Register(Scanner{}) }
