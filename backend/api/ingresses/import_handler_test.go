package ingresses

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/runner"
)

// stubRunner 与 discovery 包的 stubRunner 同样最小化：拆开 listing / cat 输出
// 派发，避免 cat 命令含 sites-enabled 路径时被误派给 listing。
type stubRunner struct {
	listingOut string
	catOut     map[string]string
}

func (s *stubRunner) Run(cmd string) (string, error) {
	if strings.HasPrefix(strings.TrimSpace(cmd), "cat ") {
		for path, out := range s.catOut {
			if strings.Contains(cmd, path) {
				return out, nil
			}
		}
		return "", nil
	}
	if strings.Contains(cmd, "ls /etc/nginx/sites-enabled") {
		return s.listingOut, nil
	}
	return "", nil
}

func (s *stubRunner) NewSession() (runner.Session, error) {
	return nil, errors.New("not impl")
}
func (s *stubRunner) IsLocal() bool      { return false }
func (s *stubRunner) Capability() string { return "full" }
func (s *stubRunner) Close() error       { return nil }

func TestImportCandidates_ReturnsParsedAndMarksAlreadyManaged(t *testing.T) {
	r, db := setup(t)
	edgeID := mkEdge(t, db)

	body := `
server {
    listen 80;
    server_name api.example.com;
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
    }
}
server {
    listen 80;
    server_name dup.example.com;
    location / {
        proxy_pass http://127.0.0.1:9000;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
`
	rn := &stubRunner{
		listingOut: "/etc/nginx/sites-enabled/app\n",
		catOut:     map[string]string{"/etc/nginx/sites-enabled/app": body},
	}
	old := SetImportRunnerFactory(func(*model.Server, *config.Config) (runner.Runner, error) {
		return rn, nil
	})
	t.Cleanup(func() { SetImportRunnerFactory(old) })

	// 预先在 DB 里塞一条 dup.example.com 的 Ingress——应被标记 AlreadyManaged。
	dup := model.Ingress{EdgeServerID: edgeID, MatchKind: "domain", Domain: "dup.example.com"}
	if err := db.Create(&dup).Error; err != nil {
		t.Fatalf("seed ingress: %v", err)
	}

	w, out := do(t, r, http.MethodGet,
		"/ingresses/edges/"+strconv.FormatUint(uint64(edgeID), 10)+"/import-candidates", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%v", w.Code, out)
	}
	data, _ := out["data"].(map[string]any)
	cands, _ := data["candidates"].([]any)
	if len(cands) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(cands))
	}
	// 找到 dup 候选，断言 already_managed=true；api 候选 already_managed=false。
	var apiAM, dupAM bool
	for _, raw := range cands {
		c := raw.(map[string]any)
		switch c["server_name"] {
		case "api.example.com":
			apiAM = c["already_managed"] == true
		case "dup.example.com":
			dupAM = c["already_managed"] == true
			// 同时校验 websocket 透传
			rs := c["routes"].([]any)
			if len(rs) != 1 || rs[0].(map[string]any)["websocket"] != true {
				t.Errorf("dup 路由 websocket 应=true: %+v", rs)
			}
		}
	}
	if apiAM {
		t.Errorf("api.example.com 不该 AlreadyManaged")
	}
	if !dupAM {
		t.Errorf("dup.example.com 应 AlreadyManaged")
	}
}

func TestImportCandidates_EdgeNotFound(t *testing.T) {
	r, _ := setup(t)
	w, _ := do(t, r, http.MethodGet,
		"/ingresses/edges/9999/import-candidates", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestImportCandidates_RunnerError(t *testing.T) {
	r, db := setup(t)
	edgeID := mkEdge(t, db)
	old := SetImportRunnerFactory(func(*model.Server, *config.Config) (runner.Runner, error) {
		return nil, errors.New("ssh down")
	})
	t.Cleanup(func() { SetImportRunnerFactory(old) })
	w, out := do(t, r, http.MethodGet,
		"/ingresses/edges/"+strconv.FormatUint(uint64(edgeID), 10)+"/import-candidates", nil)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%v", w.Code, out)
	}
}
