package cmdbuild

import (
	"errors"
	"fmt"
	"strings"

	"github.com/serverhub/serverhub/model"
)

// BuildFetchPart 按 provider 生成"把制品弄到 workdir"的 shell 段。
// 行为与 v1 pkg/deployer.buildFetchPart 字节级等价。
func BuildFetchPart(art model.Artifact) (string, error) {
	switch art.Provider {
	case model.ArtifactProviderDocker:
		if art.Ref == "" {
			return "", errors.New("docker artifact ref empty")
		}
		return fmt.Sprintf("docker pull %s 2>&1", ShellQuote(art.Ref)), nil
	case model.ArtifactProviderHTTP:
		if art.Ref == "" {
			return "", errors.New("http artifact ref empty")
		}
		dst := "artifact.bin"
		return fmt.Sprintf("curl -fsSL -o %s %s", ShellQuote(dst), ShellQuote(art.Ref)), nil
	case model.ArtifactProviderScript:
		if art.PullScript == "" {
			return "", errors.New("script artifact pull_script empty")
		}
		return art.PullScript, nil
	case model.ArtifactProviderUpload:
		if art.Ref == "" {
			return "", errors.New("upload artifact ref empty")
		}
		return fmt.Sprintf("test -f %s || (echo 'upload artifact missing on target; SFTP push not implemented in M1'; exit 1)",
			ShellQuote(art.Ref)), nil
	case model.ArtifactProviderGit:
		if art.Ref == "" {
			return "", errors.New("git artifact ref empty (expect 'repo_url' or 'repo_url@ref')")
		}
		repo, ref := parseGitRef(art.Ref)
		dir := "src"
		if ref == "" {
			return fmt.Sprintf(
				"if [ -d %s/.git ]; then cd %s && git fetch --all --prune && git reset --hard @{u} && cd ..; else rm -rf %s && git clone --depth 1 %s %s; fi",
				ShellQuote(dir), ShellQuote(dir), ShellQuote(dir), ShellQuote(repo), ShellQuote(dir),
			), nil
		}
		return fmt.Sprintf(
			"if [ -d %s/.git ]; then cd %s && git fetch --all --prune && git checkout %s && git reset --hard %s && cd ..; else rm -rf %s && git clone %s %s && cd %s && git checkout %s && cd ..; fi",
			ShellQuote(dir), ShellQuote(dir), ShellQuote(ref), ShellQuote(ref),
			ShellQuote(dir), ShellQuote(repo), ShellQuote(dir), ShellQuote(dir), ShellQuote(ref),
		), nil
	}
	return "", fmt.Errorf("unsupported provider: %s", art.Provider)
}

// parseGitRef 把 Artifact.Ref 切成 (repo, ref)。约定:
//   - 优先 '#' 分隔(避免 scp 式 user@host:path 中的 '@' 歧义)
//   - 否则按最后一个 '@' 拆,但 scp 式地址(no '://' + 含 ':') 视为整体
func parseGitRef(s string) (repo, ref string) {
	if i := strings.LastIndex(s, "#"); i >= 0 {
		return s[:i], s[i+1:]
	}
	if !strings.Contains(s, "://") && strings.Contains(s, "@") && strings.Contains(s, ":") {
		return s, ""
	}
	if i := strings.LastIndex(s, "@"); i >= 0 {
		return s[:i], s[i+1:]
	}
	return s, ""
}
