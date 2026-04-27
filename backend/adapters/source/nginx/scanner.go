package nginx

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/serverhub/serverhub/core/source"
	"github.com/serverhub/serverhub/infra"
	"github.com/serverhub/serverhub/internal/stepkit"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/safeshell"
)

const Kind = "nginx"

type Scanner struct{}

func (Scanner) Kind() string { return Kind }

// osDefaultWebRoots 是发行版 nginx/apache 默认的 welcome 页路径,用户基本
// 不会在这里部署真实站点,过滤掉。
var osDefaultWebRoots = map[string]bool{
	"/var/www":              true,
	"/var/www/html":         true,
	"/usr/share/nginx/html": true,
	"/usr/share/nginx":      true,
}

// Discover 扫 sites-enabled + conf.d 下所有 vhost,产出 type=static 候选。
// 反代 vhost(无 root,只有 proxy_pass)留给 R5 ingress adapter。
func (s Scanner) Discover(ctx context.Context, r infra.Runner) ([]source.Candidate, error) {
	listOut, _, err := r.Run(ctx,
		`( ls /etc/nginx/sites-enabled/ 2>/dev/null | sed 's|^|/etc/nginx/sites-enabled/|'; `+
			`ls /etc/nginx/conf.d/*.conf 2>/dev/null ) | sort -u`)
	if err != nil {
		return nil, nil
	}
	listOut = strings.TrimSpace(listOut)
	if listOut == "" {
		return nil, nil
	}

	var out []source.Candidate
	seen := map[string]bool{}
	for _, path := range strings.Split(listOut, "\n") {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		body, _, berr := r.Run(ctx, "cat "+safeshell.Quote(path)+" 2>/dev/null")
		if berr != nil || body == "" {
			continue
		}
		sites := parseSites(body)
		name := baseName(path)
		isDefaultFile := name == "default"
		for i, st := range sites {
			roots := st.Roots()
			if len(roots) == 0 {
				continue
			}
			sn := strings.TrimSpace(st.ServerName)
			if sn == "_" || (isDefaultFile && (sn == "" || sn == "_")) {
				continue
			}
			roots = filterStaticRoots(ctx, r, roots)
			if len(roots) == 0 {
				continue
			}
			sidBase := name
			if len(sites) > 1 {
				sidBase = name + "#" + strconv.Itoa(i)
			}
			for _, rootPath := range roots {
				sid := sidBase + "|" + slugPath(rootPath)
				if seen[sid] {
					continue
				}
				seen[sid] = true
				sum := strings.TrimSpace(st.ServerName)
				if sum == "" {
					sum = "static site"
				}
				sum += "  root=" + rootPath
				if st.HasProxy {
					sum += "  +reverse-proxy"
				}
				dispName := fallbackStr(st.ServerName, name)
				if len(roots) > 1 {
					dispName += " [" + rootPath + "]"
				}
				out = append(out, source.Candidate{
					Kind:     Kind,
					SourceID: sid,
					Name:     dispName,
					Summary:  truncate(sum, 200),
					Suggested: source.SuggestedFields{
						Type:    model.ServiceTypeStatic,
						Workdir: rootPath,
					},
					Raw: map[string]string{
						"config_file":     path,
						"server_name":     st.ServerName,
						"listen":          st.Listen,
						"all_roots":       strings.Join(roots, ","),
						"has_proxy":       boolStr(st.HasProxy),
						"location_prefix": "", // R4: 当前 nginx 静态接管未携带 location_prefix
					},
				})
			}
		}
	}
	return out, nil
}

// filterStaticRoots 剔除 OS 默认 welcome 路径与无 index.html 的目录。
// index.html 探针是"这里部署了一个真实前端"的代理信号(SPA/Vite/Webpack/Next-export
// 都会在根放一份)。
func filterStaticRoots(ctx context.Context, r infra.Runner, roots []string) []string {
	var out []string
	for _, p := range roots {
		clean := strings.TrimRight(p, "/")
		if clean == "" || osDefaultWebRoots[clean] {
			continue
		}
		stdout, _, err := r.Run(ctx, "test -f "+safeshell.Quote(clean+"/index.html")+" && echo ok")
		if err != nil || !strings.Contains(stdout, "ok") {
			continue
		}
		out = append(out, p)
	}
	return out
}

// Fingerprint: sha1("nginx|<server_name>|<location_prefix>|<workdir>"),
// 与 v1 pkg/discovery.Fingerprint(KindNginx) 字节一致。
func (Scanner) Fingerprint(c source.Candidate) string {
	key := strings.Join([]string{
		"nginx",
		c.Raw["server_name"],
		c.Raw["location_prefix"],
		c.Suggested.Workdir,
	}, "|")
	sum := sha1.Sum([]byte(key))
	return hex.EncodeToString(sum[:])
}

// Takeover 平移 v1 pkg/takeover/static.go runStatic。
//
// Flow:
//  1. precheck       - oldRoot 可读 + index.html 存在 + conf 可读
//  2. backup conf    - cat + 写到 /opt/serverhub/backups/nginx/<ts>-<base>.conf
//  3. mkdir+cp+ln    - target/releases/<ts>/, cp -a, ln -sfn current
//  4. rewrite conf   - root → target/current(嵌套 root 改 alias 避免 prefix 重叠)
//  5. nginx -t       - 配置语法校验
//  6. reload/start   - pgrep nginx 决定 reload 还是 systemctl start
//  7. probe          - curl --resolve server_name:port:127.0.0.1
//  8. rename oldRoot - mv → oldRoot.serverhub-takeover-<ts>(留作审计,不删)
func (Scanner) Takeover(ctx context.Context, tc source.TakeoverContext) error {
	cand := tc.Cand
	oldRoot := strings.TrimRight(cand.Suggested.Workdir, "/")
	confFile := cand.Raw["config_file"]
	serverName := cand.Raw["server_name"]
	listen := cand.Raw["listen"]

	if err := safeshell.AbsPath(oldRoot); err != nil {
		return fmt.Errorf("oldRoot 非法: %w", err)
	}
	if err := safeshell.AbsPath(confFile); err != nil {
		return fmt.Errorf("config_file 非法: %w", err)
	}
	if err := safeshell.ValidName(tc.SvcName, 64); err != nil {
		return fmt.Errorf("svc_name 非法: %w", err)
	}
	target := stepkit.TargetDir(tc.SvcName)
	ts := stepkit.Timestamp()
	releaseDir := target + "/releases/" + ts
	backupDir := stepkit.BackupDir("nginx")
	backupConf := backupDir + "/" + ts + "-" + confBase(confFile)

	port := parseListenPort(listen)
	probeName := serverName
	if probeName == "" || probeName == "_" {
		probeName = "localhost"
	}

	log := &stepkit.Log{}
	var (
		origConfBody string
		oldRootBak   string
	)

	steps := []stepkit.Step{
		{
			Name: "precheck: oldRoot 可读 + index.html 存在",
			Do: func() error {
				if err := stepkit.EnsureReadable(ctx, tc.Runner, oldRoot); err != nil {
					return err
				}
				stdout, _, _ := tc.Runner.Run(ctx, "test -f "+safeshell.Quote(oldRoot+"/index.html")+" && echo ok")
				if !strings.Contains(stdout, "ok") {
					return fmt.Errorf("%s/index.html 不存在", oldRoot)
				}
				return stepkit.EnsureReadable(ctx, tc.Runner, confFile)
			},
		},
		{
			Name: "读取并备份 nginx 配置",
			Do: func() error {
				out, err := stepkit.MustRun(ctx, tc.Runner, log, "cat "+safeshell.Quote(confFile))
				if err != nil {
					return err
				}
				origConfBody = out
				if _, err := stepkit.MustRun(ctx, tc.Runner, log, "sudo -n mkdir -p "+safeshell.Quote(backupDir)); err != nil {
					return err
				}
				return stepkit.WriteRemoteFile(ctx, tc.Runner, log, backupConf, origConfBody)
			},
			Undo: func() error {
				_, err := stepkit.MustRun(ctx, tc.Runner, log, "sudo -n rm -f "+safeshell.Quote(backupConf))
				return err
			},
		},
		{
			Name: "创建标准目录并复制内容",
			Do: func() error {
				cmds := []string{
					"sudo -n mkdir -p " + safeshell.Quote(releaseDir),
					"sudo -n cp -a " + safeshell.Quote(oldRoot+"/.") + " " + safeshell.Quote(releaseDir+"/"),
					"sudo -n ln -sfn " + safeshell.Quote("releases/"+ts) + " " + safeshell.Quote(target+"/current"),
					"sudo -n chmod -R a+rX " + safeshell.Quote(target),
				}
				for _, c := range cmds {
					if _, err := stepkit.MustRun(ctx, tc.Runner, log, c); err != nil {
						return err
					}
				}
				return nil
			},
			Undo: func() error {
				_, err := stepkit.MustRun(ctx, tc.Runner, log, "sudo -n rm -rf "+safeshell.Quote(target))
				return err
			},
		},
		{
			Name: "改写 nginx 配置: root → " + target + "/current",
			Do: func() error {
				newBody, hits, err := NginxRewrite(origConfBody, oldRoot, target+"/current")
				if err != nil {
					return err
				}
				log.Printf("替换 %d 处 root/alias 指令\n", hits)
				return stepkit.WriteRemoteFile(ctx, tc.Runner, log, confFile, newBody)
			},
			Undo: func() error {
				if err := stepkit.WriteRemoteFile(ctx, tc.Runner, log, confFile, origConfBody); err != nil {
					return err
				}
				_ = nginxReloadOrStart(ctx, tc.Runner, log)
				return nil
			},
		},
		{
			Name: "nginx -t 校验",
			Do: func() error {
				_, err := stepkit.MustRun(ctx, tc.Runner, log, "sudo -n nginx -t")
				return err
			},
		},
		{
			Name: "nginx reload(停机则 systemctl start)",
			Do: func() error {
				return nginxReloadOrStart(ctx, tc.Runner, log)
			},
			Undo: func() error {
				return nginxReloadOrStart(ctx, tc.Runner, log)
			},
		},
		{
			Name: "HTTP 探活 " + probeName,
			Do: func() error {
				return stepkit.ProbeHTTP(ctx, tc.Runner, log, probeName, port)
			},
		},
		{
			Name: "改名原目录为 " + oldRoot + ".serverhub-takeover-<ts>",
			Do: func() error {
				oldRootBak = oldRoot + ".serverhub-takeover-" + ts
				_, err := stepkit.MustRun(ctx, tc.Runner, log,
					"sudo -n mv "+safeshell.Quote(oldRoot)+" "+safeshell.Quote(oldRootBak))
				return err
			},
			Undo: func() error {
				if oldRootBak == "" {
					return nil
				}
				_, err := stepkit.MustRun(ctx, tc.Runner, log,
					"sudo -n mv "+safeshell.Quote(oldRootBak)+" "+safeshell.Quote(oldRoot))
				return err
			},
		},
	}
	return stepkit.RunSteps(log, steps)
}

// parseListenPort 抽 nginx listen 指令的端口数字。支持 "80"、"443 ssl"、
// "[::]:8080"、"0.0.0.0:8443 ssl"。无法解析时回落 80。
func parseListenPort(listen string) int {
	listen = strings.TrimSpace(listen)
	if listen == "" {
		return 80
	}
	first := strings.Fields(listen)[0]
	if i := strings.LastIndexByte(first, ':'); i >= 0 {
		first = first[i+1:]
	}
	if n, err := strconv.Atoi(first); err == nil && n > 0 && n < 65536 {
		return n
	}
	if strings.Contains(listen, "ssl") || strings.Contains(listen, "443") {
		return 443
	}
	return 80
}

// nginxReloadOrStart 优先 reload,nginx 未运行时改用 systemctl start。
func nginxReloadOrStart(ctx context.Context, r infra.Runner, log *stepkit.Log) error {
	out, _, _ := r.Run(ctx, "pgrep -x nginx || true")
	if strings.TrimSpace(out) != "" {
		_, err := stepkit.MustRun(ctx, r, log, "sudo -n nginx -s reload")
		return err
	}
	log.Printf("nginx 未在运行,改用 systemctl start nginx\n")
	_, err := stepkit.MustRun(ctx, r, log, "sudo -n systemctl start nginx")
	return err
}
