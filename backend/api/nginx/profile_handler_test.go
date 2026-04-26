package nginx

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
)

// 仅覆盖 GET / PUT 路径——POST /probe 需要远端 runner，已由 pkg/nginxops.ParseNginxV
// 单测覆盖，这里不重复。

func setupProfile(t *testing.T) (*gin.Engine, *gorm.DB, uint) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Server{}, &model.NginxProfile{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	s := model.Server{Name: "edge", Host: "h"}
	db.Create(&s)
	r := gin.New()
	g := r.Group("/servers")
	RegisterProfileRoutes(g, db, &config.Config{})
	return r, db, s.ID
}

func doJSON(t *testing.T, r *gin.Engine, method, path string, body any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var out map[string]any
	if w.Body.Len() > 0 {
		_ = json.Unmarshal(w.Body.Bytes(), &out)
	}
	return w, out
}

func TestProfile_GetReturnsDefaultsWhenAbsent(t *testing.T) {
	r, _, id := setupProfile(t)
	w, out := doJSON(t, r, http.MethodGet, "/servers/"+u(id)+"/nginx/profile", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%v", w.Code, out)
	}
	data, _ := out["data"].(map[string]any)
	eff, _ := data["effective"].(map[string]any)
	if eff["nginx_conf_dir"] != "/etc/nginx" {
		t.Errorf("effective.nginx_conf_dir 应回退默认 /etc/nginx，得 %v", eff["nginx_conf_dir"])
	}
	if eff["test_cmd"] != "sudo -n nginx -t 2>&1" {
		t.Errorf("effective.test_cmd 兜底失败：%v", eff["test_cmd"])
	}
	// 用户覆盖项应为空
	if data["nginx_conf_dir"] != "" {
		t.Errorf("无 profile 时用户字段应空，得 %v", data["nginx_conf_dir"])
	}
}

func TestProfile_PutCreatesAndMergesEffective(t *testing.T) {
	r, db, id := setupProfile(t)

	body := ProfileUpdateBody{
		NginxConfDir:   "/opt/nginx",
		TestCmd:        "sudo -n /opt/nginx/sbin/nginx -t 2>&1",
		ReloadCmd:      "sudo -n systemctl reload nginx 2>&1",
		HubSiteName:    "shub-hub",
		StreamsConf:    "/opt/nginx/streams.conf",
		NginxConfPath:  "/opt/nginx/nginx.conf",
	}
	w, out := doJSON(t, r, http.MethodPut, "/servers/"+u(id)+"/nginx/profile", body)
	if w.Code != http.StatusOK {
		t.Fatalf("put status=%d body=%v", w.Code, out)
	}
	data, _ := out["data"].(map[string]any)
	eff, _ := data["effective"].(map[string]any)
	if eff["nginx_conf_dir"] != "/opt/nginx" {
		t.Errorf("effective.nginx_conf_dir 未生效：%v", eff)
	}
	// 未覆盖的字段仍走默认
	if eff["sites_available_dir"] != "/etc/nginx/sites-available" {
		t.Errorf("未覆盖字段应走默认: %v", eff["sites_available_dir"])
	}
	// DB 入库
	var np model.NginxProfile
	if err := db.Where("edge_server_id = ?", id).First(&np).Error; err != nil {
		t.Fatalf("DB 未写入：%v", err)
	}
	if np.NginxConfDir != "/opt/nginx" || np.HubSiteName != "shub-hub" {
		t.Errorf("DB 字段错: %+v", np)
	}
}

func TestProfile_PutRejectsRelativePath(t *testing.T) {
	r, _, id := setupProfile(t)
	body := ProfileUpdateBody{NginxConfDir: "relative/path"}
	w, _ := doJSON(t, r, http.MethodPut, "/servers/"+u(id)+"/nginx/profile", body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("应 400 拒绝相对路径，得 %d", w.Code)
	}
}

func TestProfile_PutSecondTimeUpdates(t *testing.T) {
	r, db, id := setupProfile(t)
	doJSON(t, r, http.MethodPut, "/servers/"+u(id)+"/nginx/profile",
		ProfileUpdateBody{NginxConfDir: "/opt/a"})
	doJSON(t, r, http.MethodPut, "/servers/"+u(id)+"/nginx/profile",
		ProfileUpdateBody{NginxConfDir: "/opt/b"})

	var np model.NginxProfile
	db.Where("edge_server_id = ?", id).First(&np)
	if np.NginxConfDir != "/opt/b" {
		t.Errorf("应覆盖更新，得 %q", np.NginxConfDir)
	}
	// 不应产生重复行
	var n int64
	db.Model(&model.NginxProfile{}).Where("edge_server_id = ?", id).Count(&n)
	if n != 1 {
		t.Errorf("应只 1 行，得 %d", n)
	}
}

func u(id uint) string { return itoa(id) }

func itoa(id uint) string {
	// 简单整数转字符串，避免引 strconv 单纯为测试用
	if id == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for id > 0 {
		i--
		buf[i] = byte('0' + id%10)
		id /= 10
	}
	return string(buf[i:])
}
