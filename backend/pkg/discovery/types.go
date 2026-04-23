// Package discovery scans a server for running services (Docker containers,
// docker-compose projects, systemd units) and produces Candidate records that
// the user can selectively import as Deploy resources.
package discovery

// Kind enumerates supported service-source kinds.
const (
	KindDocker  = "docker"
	KindCompose = "compose"
	KindSystemd = "systemd"
	KindNginx   = "nginx"
)

// Candidate is a normalized representation of a discovered service. The
// Suggested map is merged directly into a Deploy at import time.
type Candidate struct {
	Kind        string            `json:"kind"`
	SourceID    string            `json:"source_id"` // stable identifier for dedup
	Name        string            `json:"name"`
	Summary     string            `json:"summary"` // short one-line human blurb
	Suggested   SuggestedDeploy   `json:"suggested"`
	ExtraLabels map[string]string `json:"extra_labels,omitempty"`

	// 由 discovery.Fingerprint(c) 填充；发现接口会查已有 Service.SourceFingerprint
	// 判断候选是否已被接管，标记到 AlreadyManaged。前端据此灰化"接管"按钮。
	Fingerprint    string `json:"fingerprint,omitempty"`
	AlreadyManaged bool   `json:"already_managed,omitempty"`
}

// SuggestedDeploy is the subset of Deploy fields the importer fills in.
type SuggestedDeploy struct {
	Type        string  `json:"type"`
	WorkDir     string  `json:"work_dir"`
	ComposeFile string  `json:"compose_file,omitempty"`
	StartCmd    string  `json:"start_cmd,omitempty"`
	ImageName   string  `json:"image_name,omitempty"`
	Runtime     string  `json:"runtime,omitempty"`
	Env         []EnvKV `json:"env,omitempty"`
}

// EnvKV is a single discovered environment variable. Secret is set by the
// scanner when the key name suggests the value is sensitive (password, token,
// secret, key, jdbc, dsn, etc.), so the UI can mask it by default.
type EnvKV struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Secret bool   `json:"secret,omitempty"`
}

// Result bundles scan output across all detectors.
type Result struct {
	Docker  []Candidate `json:"docker"`
	Compose []Candidate `json:"compose"`
	Systemd []Candidate `json:"systemd"`
	Nginx   []Candidate `json:"nginx"`
	Errors  []string    `json:"errors,omitempty"`
}
