package discovery

import (
	"github.com/serverhub/serverhub/pkg/runner"
)

// Scan runs all detectors in sequence. Detector errors are collected as
// non-fatal strings so one failure (e.g. docker not installed) doesn't block
// the others.
func Scan(rn runner.Runner, kinds []string) Result {
	want := map[string]bool{}
	for _, k := range kinds {
		want[k] = true
	}
	if len(want) == 0 {
		want[KindDocker] = true
		want[KindCompose] = true
		want[KindSystemd] = true
		want[KindNginx] = true
	}

	var r Result
	if want[KindDocker] || want[KindCompose] {
		d, c, err := ScanDocker(rn)
		if err != nil {
			r.Errors = append(r.Errors, "docker: "+err.Error())
		}
		if want[KindDocker] {
			r.Docker = d
		}
		if want[KindCompose] {
			r.Compose = c
		}
	}
	if want[KindSystemd] {
		s, err := ScanSystemd(rn)
		if err != nil {
			r.Errors = append(r.Errors, "systemd: "+err.Error())
		}
		r.Systemd = s
	}
	if want[KindNginx] {
		n, err := ScanNginx(rn)
		if err != nil {
			r.Errors = append(r.Errors, "nginx: "+err.Error())
		}
		r.Nginx = n
	}
	return r
}
