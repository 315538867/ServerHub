// n1lint: 解析 GORM slow query log，检测 N+1 查询问题。
//
// 输入为 GORM 慢日志文件（由 GORM Logger 的 SlowThreshold 输出）。
// 按请求边界将 SQL 分组，统计每组 SQL 数，超过阈值则报违规。
//
// 支持两种分组模式:
//   - window  按时间窗口分组（连续 SQL 间隔 ≤ windowMs 视为同一请求）
//   - auto    自动检测请求边界（默认，根据 SQL 间隔自动判断）
//
// 用法:
//
//	go run tools/n1lint <slow-query.log>
//	go run tools/n1lint -threshold=5 <slow-query.log>
//	go run tools/n1lint -window=200 <slow-query.log>
//
// 返回: 0 = 通过, 1 = 检测到 N+1 或 SQL 过多
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	threshold = flag.Int("threshold", 5, "单请求允许的最大 SQL 数")
	window    = flag.Int("window", 0, "时间窗口(ms),0=自动检测请求边界")
)

// gormLogLineRE 匹配 GORM 慢日志行:
//
//	2024/01/01 12:00:00 /path/file.go:123
//	[123.456ms] [rows:1] SELECT * FROM users WHERE id = 1;
var (
	logTimeRE = regexp.MustCompile(`^(\d{4}/\d{2}/\d{2}\s+\d{2}:\d{2}:\d{2})`)
	sqlRowRE  = regexp.MustCompile(`^\[(\d+\.?\d*)ms\]\s+\[rows:\d+\]\s+(.+)`)
)

type sqlEntry struct {
	Time time.Time
	SQL  string
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: n1lint [flags] <slow-query.log>\n\n")
		fmt.Fprintf(os.Stderr, "flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(0)
	}

	entries, err := parseLog(flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "n1lint: parse error: %v\n", err)
		os.Exit(1)
	}
	if len(entries) == 0 {
		fmt.Println("n1lint: no slow queries found")
		os.Exit(0)
	}

	groups := groupEntries(entries)
	violations := checkGroups(groups)

	if len(violations) > 0 {
		fmt.Printf("n1lint: %d request(s) exceed threshold (max %d SQL)\n\n",
			len(violations), *threshold)
		for _, v := range violations {
			fmt.Printf("  request at %s — %d SQL\n",
				v.Time.Format("15:04:05.000"), v.Count)
			for _, s := range v.SQLs {
				fmt.Printf("    %s\n", truncateSQL(s, 120))
			}
			fmt.Println()
		}
		fmt.Printf("n1lint: FAIL (%d violations)\n", len(violations))
		os.Exit(1)
	}

	fmt.Printf("n1lint: clean — %d requests, max %d SQL/req (threshold=%d)\n",
		len(groups), maxSQLPerGroup(groups), *threshold)
}

type group struct {
	Time time.Time
	SQLs []string
	Count int
}

func parseLog(path string) ([]sqlEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []sqlEntry
	var currentTime time.Time
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := scanner.Text()

		// 跳过空行和注释行
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 尝试匹配时间戳行
		if m := logTimeRE.FindStringSubmatch(line); m != nil {
			t, err := time.Parse("2006/01/02 15:04:05", m[1])
			if err == nil {
				currentTime = t
			}
			continue
		}

		// 尝试匹配 SQL 行
		if m := sqlRowRE.FindStringSubmatch(line); m != nil {
			sql := strings.TrimSpace(m[2])
			if sql != "" && !currentTime.IsZero() {
				entries = append(entries, sqlEntry{
					Time: currentTime,
					SQL:  sql,
				})
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Time.Before(entries[j].Time)
	})

	return entries, nil
}

func groupEntries(entries []sqlEntry) []group {
	if len(entries) == 0 {
		return nil
	}

	// 自动导出窗口大小: 取"第一条 SQL 到本节最后一条"的时间跨度中位数间隔
	// 若用户提供了 -window,则直接使用。
	windowMs := *window
	if windowMs <= 0 {
		windowMs = autoWindow(entries)
	}
	windowDur := time.Duration(windowMs) * time.Millisecond

	var groups []group
	cur := group{Time: entries[0].Time}
	for _, e := range entries {
		if e.Time.Sub(cur.Time) > windowDur {
			groups = append(groups, cur)
			cur = group{Time: e.Time}
		}
		cur.SQLs = append(cur.SQLs, e.SQL)
		cur.Count++
	}
	groups = append(groups, cur)
	return groups
}

// autoWindow 根据入口间的最大间隔自动判定请求边界。
// 策略: 取相邻 SQL 间隔的中位数的 3 倍作为窗口——正常情况下同请求内 SQL
// 间隔很小（sub-ms 到数 ms），跨请求则通常 ≥ 序列化/网络延迟（数 10 ms）。
func autoWindow(entries []sqlEntry) int {
	if len(entries) < 2 {
		return 100 // 单条无法判断,给一个保守默认值
	}

	var gaps []int
	for i := 1; i < len(entries); i++ {
		gap := int(entries[i].Time.Sub(entries[i-1].Time).Milliseconds())
		if gap >= 0 {
			gaps = append(gaps, gap)
		}
	}
	if len(gaps) == 0 {
		return 100
	}

	sort.Ints(gaps)
	median := gaps[len(gaps)/2]
	// 3x 中位数, 最小 50ms, 最大 2s
	win := median * 3
	if win < 50 {
		win = 50
	}
	if win > 2000 {
		win = 2000
	}
	return win
}

type violation struct {
	Time  time.Time
	Count int
	SQLs  []string
}

func checkGroups(groups []group) []violation {
	var out []violation
	for _, g := range groups {
		if g.Count > *threshold {
			out = append(out, violation{
				Time:  g.Time,
				Count: g.Count,
				SQLs:  g.SQLs,
			})
		}
	}
	return out
}

func maxSQLPerGroup(groups []group) int {
	m := 0
	for _, g := range groups {
		if g.Count > m {
			m = g.Count
		}
	}
	return m
}

func truncateSQL(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > n {
		return s[:n-3] + "..."
	}
	return s
}

// printVersion 打印版本信息（供 Makefile 引用）。
func printVersion() {
	fmt.Println("n1lint v0.6.0 (R6)")
}

func init() {
	// 检测 --version 标志
	for _, a := range os.Args[1:] {
		if a == "--version" || a == "-version" {
			printVersion()
			os.Exit(0)
		}
	}
}

