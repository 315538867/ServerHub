package takeover

import (
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/runner"
	"github.com/serverhub/serverhub/pkg/safeshell"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

// runDocker takes over a standalone container by materializing it as a
// docker-compose project under the standard apps directory, then bringing the
// new compose stack up and the old container down (renamed, not removed —
// operator audits).
//
// Bind mounts have their source directories copied into target/data/<basename>
// so the new stack is self-contained. Named volumes are referenced by name and
// survive untouched (docker volume inspect returns the same data). Networks
// are recreated by compose; --network=host is preserved.
//
// Flow (plan §Docker container):
//  1. inspect      - read the container's config snapshot
//  2. precheck     - target absent; bind sources readable
//  3. materialize  - mkdir target + target/data; cp -a binds; write compose yml
//  4. stop+rename  - docker stop <id>; docker rename <id> <name>-pre-takeover-<ts>
//  5. compose up   - cd target && docker compose up -d
//  6. probe        - all services running
//  7. db insert
//
// Rollback walks reverse: compose down → restore renamed container → start old.
func runDocker(db *gorm.DB, rn runner.Runner, log *Log, server model.Server,
	req Request, res *Result) error {

	c := req.Candidate
	containerID := c.SourceID
	if containerID == "" {
		return fmt.Errorf("候选缺少 source_id (container id)")
	}

	target := TargetDir(req.TargetName)
	ts := Timestamp()
	dataDir := target + "/data"
	composePath := target + "/docker-compose.yml"
	preTakeoverName := req.TargetName + "-pre-takeover-" + ts

	var (
		spec      containerSpec
		bindCopy  []bindRewrite // src on host → relative dataDir entry
		composeYM string
		oldName   string
	)

	steps := []Step{
		{
			Name: "docker inspect 源容器",
			Do: func() error {
				out, err := MustRun(rn, log,
					"sudo -n docker inspect "+safeshell.Quote(containerID))
				if err != nil {
					return err
				}
				s, err := parseContainerSpec(out)
				if err != nil {
					return err
				}
				spec = s
				oldName = strings.TrimPrefix(spec.Name, "/")
				return nil
			},
		},
		{
			Name: "precheck: bind mount 源路径可读",
			Do: func() error {
				for _, m := range spec.Mounts {
					if m.Type != "bind" {
						continue
					}
					if err := EnsureReadable(rn, m.Source); err != nil {
						return fmt.Errorf("bind 源不可读 %s: %w", m.Source, err)
					}
				}
				return nil
			},
		},
		{
			Name: "建立目标目录并复制 bind 数据",
			Do: func() error {
				if _, err := MustRun(rn, log,
					"sudo -n mkdir -p "+safeshell.Quote(dataDir)); err != nil {
					return err
				}
				used := map[string]int{}
				for _, m := range spec.Mounts {
					if m.Type != "bind" {
						continue
					}
					base := uniqueBindName(path.Base(strings.TrimRight(m.Source, "/")), used)
					dst := dataDir + "/" + base
					_, err := MustRun(rn, log,
						"sudo -n cp -a "+safeshell.Quote(m.Source+"/.")+" "+safeshell.Quote(dst+"/"))
					// cp from a file (not directory) needs different syntax
					if err != nil {
						// retry as plain file copy
						_, err = MustRun(rn, log,
							"sudo -n cp -a "+safeshell.Quote(m.Source)+" "+safeshell.Quote(dst))
					}
					if err != nil {
						return err
					}
					bindCopy = append(bindCopy, bindRewrite{
						Original:    m.Source,
						RelInTarget: "./data/" + base,
						Destination: m.Destination,
						ReadOnly:    !m.RW,
					})
				}
				return nil
			},
			Undo: func() error {
				_, err := MustRun(rn, log, "sudo -n rm -rf "+safeshell.Quote(target))
				return err
			},
		},
		{
			Name: "生成 docker-compose.yml",
			Do: func() error {
				ym, err := buildComposeYAML(req.TargetName, spec, bindCopy)
				if err != nil {
					return err
				}
				composeYM = ym
				return runtimeWriteFile(rn, log, composePath, composeYM)
			},
			// no separate undo — the cleanup of `target` in step 3's undo handles it
		},
		{
			Name: "停止并重命名旧容器为 " + preTakeoverName,
			Do: func() error {
				if _, err := MustRun(rn, log,
					"sudo -n docker stop "+safeshell.Quote(containerID)); err != nil {
					return err
				}
				if _, err := MustRun(rn, log,
					"sudo -n docker rename "+safeshell.Quote(containerID)+" "+safeshell.Quote(preTakeoverName)); err != nil {
					// stop succeeded but rename failed — try to restart so the system
					// is back to its prior state, then surface the error
					_, _ = MustRun(rn, log, "sudo -n docker start "+safeshell.Quote(containerID))
					return err
				}
				return nil
			},
			Undo: func() error {
				_, _ = MustRun(rn, log,
					"sudo -n docker rename "+safeshell.Quote(preTakeoverName)+" "+safeshell.Quote(oldName))
				_, err := MustRun(rn, log,
					"sudo -n docker start "+safeshell.Quote(oldName))
				return err
			},
		},
		{
			Name: "启动新 compose 栈",
			Do: func() error {
				cmd := fmt.Sprintf("cd %s && sudo -n docker compose up -d",
					safeshell.Quote(target))
				_, err := MustRun(rn, log, cmd)
				return err
			},
			Undo: func() error {
				cmd := fmt.Sprintf("cd %s && sudo -n docker compose down",
					safeshell.Quote(target))
				_, err := MustRun(rn, log, cmd)
				return err
			},
		},
		{
			Name: "等待新栈就绪",
			Do: func() error {
				return waitComposeRunning(rn, log, target, "docker-compose.yml", composePath)
			},
		},
	}

	if err := RunSteps(log, steps); err != nil {
		res.RolledBack = true
		return err
	}

	d := model.Service{
		Name:       req.TargetName,
		ServerID:   server.ID,
		Type:       model.ServiceTypeDockerCompose,
		WorkDir:    target,
		SourceKind: c.Kind,
		SourceID:   c.SourceID,
		SyncStatus: "synced",
	}
	if _, err := attachToApplication(db, &d, c, req); err != nil {
		log.Printf("⚠ Application 绑定失败: %v\n", err)
		return fmt.Errorf("application 绑定失败: %w", err)
	}
	if err := db.Create(&d).Error; err != nil {
		log.Printf("⚠ Deploy 写入失败（主机已迁移完成）: %v\n", err)
		return fmt.Errorf("DB 写入失败: %w", err)
	}
	if d.ApplicationID != nil {
		finalizeApplicationLink(db, *d.ApplicationID, d.ID)
	}
	log.Printf("Deploy 已创建: id=%d name=%s\n", d.ID, d.Name)
	res.DeployID = d.ID
	return nil
}

// containerSpec is the subset of `docker inspect` output the takeover needs.
// Field names match docker's JSON exactly so we can json.Unmarshal directly.
type containerSpec struct {
	Name   string
	Image  string
	Config struct {
		Env          []string
		ExposedPorts map[string]struct{}
		Cmd          []string
		Entrypoint   []string
	}
	HostConfig struct {
		PortBindings map[string][]struct {
			HostIP   string `json:"HostIp"`
			HostPort string `json:"HostPort"`
		}
		RestartPolicy struct {
			Name              string
			MaximumRetryCount int
		}
		NetworkMode string
	}
	Mounts []struct {
		Type        string
		Source      string
		Destination string
		Mode        string
		RW          bool
		Name        string
	}
}

// parseContainerSpec extracts the first element of `docker inspect`'s JSON
// array. inspect always returns an array, even for a single ID.
func parseContainerSpec(raw string) (containerSpec, error) {
	var arr []containerSpec
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&arr); err != nil {
		return containerSpec{}, fmt.Errorf("inspect JSON 解析失败: %w", err)
	}
	if len(arr) == 0 {
		return containerSpec{}, fmt.Errorf("inspect 返回空数组")
	}
	return arr[0], nil
}

// bindRewrite tracks where we copied each bind source so the compose volumes
// list points into the new self-contained data dir.
type bindRewrite struct {
	Original    string
	RelInTarget string
	Destination string
	ReadOnly    bool
}

// uniqueBindName disambiguates colliding basenames (two binds whose source
// dirs share a tail like `config`) by appending a counter.
func uniqueBindName(base string, used map[string]int) string {
	if base == "" || base == "/" {
		base = "data"
	}
	if used[base] == 0 {
		used[base] = 1
		return base
	}
	used[base]++
	return fmt.Sprintf("%s-%d", base, used[base])
}

// buildComposeYAML renders a minimal compose v3 file mirroring the inspected
// container. Uses yaml.Marshal on a typed map so the output is deterministic
// and round-trips through any compose toolchain.
func buildComposeYAML(serviceName string, s containerSpec, binds []bindRewrite) (string, error) {
	svc := map[string]any{
		"image":          s.Image,
		"container_name": serviceName,
	}
	if len(s.Config.Env) > 0 {
		// emit as a list to preserve = in values verbatim
		envs := make([]string, 0, len(s.Config.Env))
		envs = append(envs, s.Config.Env...)
		sort.Strings(envs)
		svc["environment"] = envs
	}
	if len(s.Config.Cmd) > 0 {
		svc["command"] = s.Config.Cmd
	}
	if len(s.Config.Entrypoint) > 0 {
		svc["entrypoint"] = s.Config.Entrypoint
	}
	if rp := s.HostConfig.RestartPolicy.Name; rp != "" && rp != "no" {
		svc["restart"] = rp
	}
	if nm := s.HostConfig.NetworkMode; nm == "host" {
		svc["network_mode"] = "host"
	}
	if pbs := s.HostConfig.PortBindings; len(pbs) > 0 {
		var ports []string
		for cport, bs := range pbs {
			// cport is like "80/tcp"; strip the proto for the canonical short form
			c := cport
			proto := ""
			if i := strings.IndexByte(cport, '/'); i >= 0 {
				c = cport[:i]
				proto = cport[i+1:]
			}
			for _, b := range bs {
				host := b.HostPort
				if host == "" {
					continue
				}
				p := host + ":" + c
				if proto != "" && proto != "tcp" {
					p += "/" + proto
				}
				ports = append(ports, p)
			}
		}
		sort.Strings(ports)
		if len(ports) > 0 {
			svc["ports"] = ports
		}
	}
	if len(binds) > 0 {
		var vols []string
		for _, b := range binds {
			line := b.RelInTarget + ":" + b.Destination
			if b.ReadOnly {
				line += ":ro"
			}
			vols = append(vols, line)
		}
		// also surface any non-bind volume mounts (named volumes) for fidelity
		for _, m := range s.Mounts {
			if m.Type == "volume" && m.Name != "" {
				ln := m.Name + ":" + m.Destination
				if !m.RW {
					ln += ":ro"
				}
				vols = append(vols, ln)
			}
		}
		svc["volumes"] = vols
	}
	root := map[string]any{
		"services": map[string]any{
			serviceName: svc,
		},
	}
	// declare named volumes so compose creates them as external references
	named := map[string]any{}
	for _, m := range s.Mounts {
		if m.Type == "volume" && m.Name != "" {
			named[m.Name] = map[string]any{"external": true}
		}
	}
	if len(named) > 0 {
		root["volumes"] = named
	}
	out, err := yaml.Marshal(root)
	if err != nil {
		return "", err
	}
	return "# generated by serverhub takeover\n" + string(out), nil
}

// runtimeWriteFile is a thin wrapper around safeshell.WriteRemoteFile + MustRun
// so step closures stay short.
func runtimeWriteFile(rn runner.Runner, log *Log, p, content string) error {
	_, err := MustRun(rn, log, safeshell.WriteRemoteFile(p, content, true))
	return err
}
