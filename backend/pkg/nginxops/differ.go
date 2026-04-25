package nginxops

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/serverhub/serverhub/pkg/nginxrender"
	"github.com/serverhub/serverhub/pkg/runner"
	"github.com/serverhub/serverhub/pkg/safeshell"
)

// ActualFile 是 Inspect 从远端读出的一个文件快照。
type ActualFile struct {
	Path    string
	Content string
	Hash    string
}

// inspectScript 在远端列出所有「我们管的」nginx 配置文件，逐行输出
// `<path>\t<sha256>\t<base64>`。三段以 TAB 分隔，方便上层解析。
//
// 管辖范围（与 nginxrender 路径策略对齐）：
//   - SitesAvailableDir/*-sh.conf
//   - SitesAvailableDir/serverhub-app-hub
//   - AppLocationsDir/*.conf
//
// 用 `2>/dev/null || true` 容忍目录不存在的场景（首次 apply）。
const inspectScript = `set -eu
shopt -s nullglob 2>/dev/null || true
emit() {
  for f in "$@"; do
    [ -f "$f" ] || continue
    h=$(sha256sum "$f" | awk '{print $1}')
    b=$(base64 -w 0 "$f" 2>/dev/null || base64 "$f" | tr -d '\n')
    printf '%s\t%s\t%s\n' "$f" "$h" "$b"
  done
}
emit ` + nginxrender.SitesAvailableDir + `/*-sh.conf 2>/dev/null || true
emit ` + nginxrender.SitesAvailableDir + `/` + nginxrender.HubSiteName + ` 2>/dev/null || true
emit ` + nginxrender.AppLocationsDir + `/*.conf 2>/dev/null || true
exit 0
`

// Inspect 在远端枚举管辖文件并返回 path → ActualFile 映射。
// 提取出 runner 调用是为了让 Diff 保持纯函数、便于单测。
func Inspect(r runner.Runner) (map[string]ActualFile, error) {
	out, err := r.Run("bash -c " + safeshell.Quote(inspectScript))
	if err != nil {
		return nil, fmt.Errorf("inspect 失败: %s: %w", strings.TrimSpace(out), err)
	}
	return ParseInspect(out)
}

// ParseInspect 解析 inspectScript 的输出。导出便于单测。
func ParseInspect(raw string) (map[string]ActualFile, error) {
	res := map[string]ActualFile{}
	for ln, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("inspect 输出第 %d 行格式异常: %q", ln+1, line)
		}
		decoded, err := base64.StdEncoding.DecodeString(parts[2])
		if err != nil {
			return nil, fmt.Errorf("inspect 第 %d 行 base64 解码失败: %w", ln+1, err)
		}
		res[parts[0]] = ActualFile{
			Path:    parts[0],
			Content: string(decoded),
			Hash:    parts[1],
		}
	}
	return res, nil
}

// Diff 对比 desired 与 actual，输出按 Path 升序的 Change 列表。
//   - desired 中 actual 没有 → add
//   - actual 中 desired 没有 → delete
//   - 双方都有但 sha256 不同 → update
//   - 双方都有且 sha256 相同 → 跳过
//
// 纯函数：没有任何 IO。actual 由 Inspect 读取后传入。
func Diff(desired []nginxrender.ConfigFile, actual map[string]ActualFile) []Change {
	desiredMap := make(map[string]nginxrender.ConfigFile, len(desired))
	for _, f := range desired {
		desiredMap[f.Path] = f
	}

	var changes []Change

	for path, want := range desiredMap {
		newHash := sha256Hex(want.Content)
		if cur, ok := actual[path]; ok {
			if cur.Hash == newHash {
				continue
			}
			changes = append(changes, Change{
				Kind: ChangeUpdate, Path: path,
				NewContent: want.Content, NewHash: newHash,
				OldContent: cur.Content, OldHash: cur.Hash,
			})
		} else {
			changes = append(changes, Change{
				Kind: ChangeAdd, Path: path,
				NewContent: want.Content, NewHash: newHash,
			})
		}
	}

	for path, cur := range actual {
		if _, ok := desiredMap[path]; ok {
			continue
		}
		changes = append(changes, Change{
			Kind: ChangeDelete, Path: path,
			OldContent: cur.Content, OldHash: cur.Hash,
		})
	}

	sort.Slice(changes, func(i, j int) bool {
		if changes[i].Path != changes[j].Path {
			return changes[i].Path < changes[j].Path
		}
		return changes[i].Kind < changes[j].Kind
	})
	return changes
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
