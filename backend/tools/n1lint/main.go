// Package main 占位:R0 阶段产物,R6 实现完整 N+1 检查。
//
// 用法(R6 启用):
//
//	tools/n1lint <slow.log>
//
// 解析 sqlite slow log,统计每个 HTTP request 周期内的 SQL 数量。
// 任一 request SQL 数 > 阈值(默认 5)→ exit 1 + 输出违规清单。
//
// R0 阶段:仅打印 "skipped",CI 中保持 skip 状态,不阻塞构建。
package main

import (
	"fmt"
	"os"
)

const (
	exitOK         = 0
	exitViolations = 1

	// defaultThreshold 是单 request 内允许的最大 SQL 数。
	// R6 阶段会根据实际 list 接口要求调到 4。
	defaultThreshold = 5
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: n1lint <sqlite-slow.log>")
		os.Exit(exitOK) // R0 占位 skip
	}
	fmt.Println("n1lint: R0 placeholder — full implementation lands at R6")
	fmt.Printf("  would scan: %s (threshold=%d)\n", os.Args[1], defaultThreshold)
	os.Exit(exitOK)
}
