package discovery

import (
	"encoding/json"
	"strings"

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
			continue
		}
		docker = append(docker, Candidate{
			Kind:     KindDocker,
			SourceID: row.ID,
			Name:     row.Names,
			Summary:  row.Image + " (" + row.Status + ")",
			Suggested: SuggestedDeploy{
				Type:      "docker",
				ImageName: row.Image,
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
				Type:        "docker-compose",
				WorkDir:     g.workingDir,
				ComposeFile: cf,
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
