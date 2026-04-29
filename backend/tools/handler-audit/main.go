// handler-audit: 扫描 backend/api/ 非测试 .go 文件，检测 GORM 直接调用。
//
// 检查项:
//  1. import "gorm.io/gorm" — handler 不得直接导入 gorm（主要防线）
//  2. 高特异性 GORM 方法调用 — handler 应通过 repo/ 层访问 DB
//
// 为避免误报，方法列表仅包含 GORM 特有、在标准 Go handler 代码中几乎不会
// 以其他语义出现的调用（如 AutoMigrate / Pluck / Preload / Clauses 等）。
// 通用方法名如 Error / Create / Find / Where 不在检查范围内——
// 它们由 import 检查（第 1 项）间接覆盖。
//
// 用法:
//
//	go run tools/handler-audit
//
// 返回: 0 = clean, 1 = 有违规
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// gormUniqueMethods 是 GORM 高度特异的链式/终端方法。
// 这些方法名在一般的 handler 代码中不会出现（或极少以其他语义出现），
// 因此误报风险极低。通用方法（Error / Create / Find / Where 等）不在此列。
var gormUniqueMethods = map[string]bool{
	// 查询终端
	"Pluck": true, "Take": true,

	// 链式
	"Preload":   true,
	"Joins":     true,
	"Clauses":   true,
	"Unscoped":  true,
	"Attrs":     true,
	"Assign":    true,
	"Omit":      true,

	// Schema / migration
	"AutoMigrate": true,
	"Migrator":    true,
	"Association": true,

	// 事务
	"Transaction": true,
}

func main() {
	apiDir := "api"
	if _, err := os.Stat(apiDir); os.IsNotExist(err) {
		// 可能从 repo root 运行
		if _, err := os.Stat("backend/api"); err == nil {
			apiDir = "backend/api"
		} else {
			fmt.Fprintln(os.Stderr, "handler-audit: cannot find api/ directory")
			fmt.Fprintln(os.Stderr, "  run from repo root:  go -C backend run tools/handler-audit")
			fmt.Fprintln(os.Stderr, "  or from backend/:     go run tools/handler-audit")
			os.Exit(1)
		}
	}

	violations := 0
	_ = filepath.WalkDir(apiDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			fmt.Fprintf(os.Stderr, "handler-audit: parse %s: %v\n", path, err)
			return nil
		}

		// 检查 1: gorm.io/gorm import（主防线）
		for _, imp := range f.Imports {
			if strings.Contains(imp.Path.Value, "gorm.io/gorm") &&
				!strings.Contains(imp.Path.Value, "gorm.io/gorm/") {
				fmt.Printf("%s:%d: imports %s\n",
					path, fset.Position(imp.Pos()).Line, imp.Path.Value)
				violations++
			}
		}

		// 检查 2: 高特异性 GORM 方法（辅助防线）
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if gormUniqueMethods[sel.Sel.Name] {
				fmt.Printf("%s:%d: GORM-specific call .%s()\n",
					path, fset.Position(call.Pos()).Line, sel.Sel.Name)
				violations++
			}
			return true
		})
		return nil
	})

	if violations > 0 {
		fmt.Printf("\nhandler-audit: %d violation(s) found\n", violations)
		fmt.Println("  handler 不得直接调用 GORM 方法或导入 gorm.io/gorm，请使用 repo/ 层")
		os.Exit(1)
	}
	fmt.Println("handler-audit: clean (0 GORM-specific calls / 0 gorm imports in api/ handlers)")
}
