// Package main 占位:R0 阶段产物,R6 实现完整 handler 直 db 审计。
//
// 用法(R6 启用):
//
//	tools/handler-audit
//
// 扫 backend/api/**/*.go(排除 _test.go),用 go/ast 找 *gorm.DB 的方法调用。
// 任一发现 → exit 1 + 输出违规位置。
//
// R0 阶段:仅打印 "skipped"。R0 基线统计已经手工入 baseline/handler-direct-db-v1.txt。
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("handler-audit: R0 placeholder — full implementation lands at R6")
	fmt.Println("  R0 baseline snapshot at baseline/handler-direct-db-v1.txt")
	os.Exit(0)
}
