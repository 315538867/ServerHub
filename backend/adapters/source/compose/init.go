// Package compose 是 SourceScanner adapter,负责发现并接管 docker-compose
// 项目(通过 com.docker.compose.project label 聚合容器为一个候选)。
package compose

import "github.com/serverhub/serverhub/core/source"

func init() { source.Default.Register(Scanner{}) }
