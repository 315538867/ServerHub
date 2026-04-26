package discovery

import (
	"encoding/json"
	"strings"

	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/runner"
)

// dockerPS is one row of `docker ps --format '{{json .}}'`.
type dockerPS struct {
	ID     string `json:"ID"`
	Names  string `json:"Names"`
	Image  string `json:"Image"`
	Labels string `json:"Labels"`
	Status string `json:"Status"`
}

// dockerEnvSkip — env vars docker / OCI runtimes inject by default. Filtering
// them keeps the imported deploy clean: the user only sees what their image or
// `docker run -e` actually set.
var dockerEnvSkip = map[string]bool{
	"PATH": true, "HOSTNAME": true, "HOME": true, "TERM": true,
	"PWD": true, "SHLVL": true, "LANG": true,
}

// inspectContainerEnv reads `.Config.Env` from `docker inspect` and returns a
// filtered list. Errors (container vanished, docker hiccup) silently return
// nil — discovery should not abort over one missing container.
func inspectContainerEnv(rn runner.Runner, id string) []EnvKV {
	if id == "" {
		return nil
	}
	out, err := rn.Run(`docker inspect --format '{{json .Config.Env}}' ` + shellQuote(id) + ` 2>/dev/null`)
	if err != nil || strings.TrimSpace(out) == "" {
		return nil
	}
	var raw []string
	if e := json.Unmarshal([]byte(strings.TrimSpace(out)), &raw); e != nil {
		return nil
	}
	pairs := parseKVPairs(raw)
	filtered := pairs[:0]
	for _, kv := range pairs {
		if dockerEnvSkip[kv.Key] {
			continue
		}
		filtered = append(filtered, kv)
	}
	return filtered
}

// ScanDocker lists running containers and splits them into standalone docker
// candidates and docker-compose project candidates (grouped by the
// com.docker.compose.project label).
func ScanDocker(rn runner.Runner) (docker, compose []Candidate, err error) {
	out, rerr := rn.Run(`docker ps --format '{{json .}}' 2>/dev/null`)
	if rerr != nil || strings.TrimSpace(out) == "" {
		return nil, nil, rerr
	}

	type composeGroup struct {
		project    string
		workingDir string
		file       string
		images     []string
		services   []string
		env        []EnvKV
	}
	groups := map[string]*composeGroup{}

	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var row dockerPS
		if e := json.Unmarshal([]byte(line), &row); e != nil {
			continue
		}
		labels := parseLabels(row.Labels)
		env := inspectContainerEnv(rn, row.ID)
		if project := labels["com.docker.compose.project"]; project != "" {
			g, ok := groups[project]
			if !ok {
				g = &composeGroup{
					project:    project,
					workingDir: labels["com.docker.compose.project.working_dir"],
					file:       labels["com.docker.compose.project.config_files"],
				}
				groups[project] = g
			}
			g.images = append(g.images, row.Image)
			if svc := labels["com.docker.compose.service"]; svc != "" {
				g.services = append(g.services, svc)
			}
			g.env = mergeEnv(g.env, env)
			continue
		}
		docker = append(docker, Candidate{
			Kind:     KindDocker,
			SourceID: row.ID,
			Name:     row.Names,
			Summary:  row.Image + " (" + row.Status + ")",
			Suggested: SuggestedDeploy{
				Type:      model.ServiceTypeDocker,
				ImageName: row.Image,
				Env:       env,
			},
		})
	}

	for _, g := range groups {
		cf := g.file
		if idx := strings.LastIndex(cf, "/"); idx >= 0 {
			cf = cf[idx+1:]
		}
		if cf == "" {
			cf = "docker-compose.yml"
		}
		compose = append(compose, Candidate{
			Kind:     KindCompose,
			SourceID: g.project,
			Name:     g.project,
			Summary:  strings.Join(dedup(g.services), ", "),
			Suggested: SuggestedDeploy{
				Type:        model.ServiceTypeDockerCompose,
				WorkDir:     g.workingDir,
				ComposeFile: cf,
				Env:         g.env,
			},
		})
	}
	return docker, compose, nil
}

// parseLabels parses the comma-separated `k=v` string emitted by `docker ps`.
func parseLabels(s string) map[string]string {
	m := map[string]string{}
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if eq := strings.IndexByte(p, '='); eq > 0 {
			m[p[:eq]] = p[eq+1:]
		}
	}
	return m
}

func dedup(xs []string) []string {
	seen := make(map[string]struct{}, len(xs))
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		if _, ok := seen[x]; ok {
			continue
		}
		seen[x] = struct{}{}
		out = append(out, x)
	}
	return out
}
