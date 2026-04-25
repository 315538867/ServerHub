package nginxops

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/serverhub/serverhub/pkg/nginxrender"
	"github.com/serverhub/serverhub/pkg/runner"
	"github.com/serverhub/serverhub/pkg/safeshell"
)

// nginx.conf 内的标记块格式：
//
//   # >>> serverhub stream begin
//   include /etc/nginx/streams.conf;
//   # <<< serverhub stream end
//
// 用 marker 包夹是为了让幂等重写不依赖文件内容的位置，也便于排查与人工清理。
const (
	streamMarkerBegin = "# >>> serverhub stream begin"
	streamMarkerEnd   = "# <<< serverhub stream end"
	nginxConfPath     = "/etc/nginx/nginx.conf"
)

// streamMarkerRE 匹配 marker 块（含两端 marker、include 行与紧邻空行），用于剥除。
// (?ms) 让 . 匹配换行；尾部 \s* 吞掉块后的空白行避免每次重写堆叠空行。
var streamMarkerRE = regexp.MustCompile(`(?ms)(?:^[ \t]*\r?\n)?` +
	regexp.QuoteMeta(streamMarkerBegin) + `\r?\n.*?` +
	regexp.QuoteMeta(streamMarkerEnd) + `\r?\n?`)

// desiredHasStreams 在 desired 列表里检索是否存在 streams.conf。
func desiredHasStreams(desired []nginxrender.ConfigFile) bool {
	for _, f := range desired {
		if f.Path == nginxrender.StreamsConf {
			return true
		}
	}
	return false
}

// ensureStreamInclude 幂等地在远端 nginx.conf 顶层维护 stream include 标记块。
//
//   - want=true : 确保块存在（若已存在则原样保留；否则追加到文件末尾）
//   - want=false: 确保块不存在（若存在则剥除）
//
// 任何实际写入都会返回一个 ChangeUpdate 合成项，调用方追加到 rollback 列表，
// 由现有 rollback() 用 OldContent 回写。
func ensureStreamInclude(r runner.Runner, want bool) (*Change, error) {
	out, err := r.Run("sudo -n cat " + safeshell.Quote(nginxConfPath))
	if err != nil {
		return nil, fmt.Errorf("读取 nginx.conf 失败: %s: %w", strings.TrimSpace(out), err)
	}
	oldContent := out
	newContent := stripStreamMarkers(oldContent)
	if want {
		newContent = appendStreamMarkers(newContent)
	}
	if newContent == oldContent {
		return nil, nil
	}
	cmd := safeshell.WriteRemoteFile(nginxConfPath, newContent, true)
	if w, err := r.Run(cmd); err != nil {
		return nil, fmt.Errorf("写入 nginx.conf 失败: %s: %w", strings.TrimSpace(w), err)
	}
	return &Change{
		Kind:       ChangeUpdate,
		Path:       nginxConfPath,
		OldContent: oldContent,
		OldHash:    sha256Hex(oldContent),
		NewContent: newContent,
		NewHash:    sha256Hex(newContent),
	}, nil
}

// stripStreamMarkers 删除全部 marker 块（重复出现也一并清掉，防御历史污染）。
func stripStreamMarkers(s string) string {
	return streamMarkerRE.ReplaceAllString(s, "")
}

// appendStreamMarkers 把 marker 块追加到 nginx.conf 末尾。
// 始终保证文件末尾恰好一个换行后再写 marker，避免空行堆积。
func appendStreamMarkers(s string) string {
	trimmed := strings.TrimRight(s, "\n") + "\n"
	return trimmed + "\n" + streamMarkerBegin + "\n" +
		"include " + nginxrender.StreamsConf + ";\n" +
		streamMarkerEnd + "\n"
}
