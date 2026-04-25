package nginxrender

import (
	"strings"
	"testing"
)

func TestRender_DomainSingleRoute(t *testing.T) {
	files, err := Render([]IngressCtx{{
		EdgeServerID: 1,
		FileStem:     "myapp",
		MatchKind:    MatchKindDomain,
		Domain:       "app.example.com",
		Routes: []RouteCtx{{
			Sort: 0, Path: "/", UpstreamURL: "http://10.0.0.5:8080",
		}},
	}})
	if err != nil {
		t.Fatalf("Render err: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("want 1 file, got %d", len(files))
	}
	want := "/etc/nginx/sites-available/myapp-sh.conf"
	if files[0].Path != want {
		t.Errorf("path: want %q got %q", want, files[0].Path)
	}
	wantContent := `server {
    listen 80;
    server_name app.example.com;

    location / {
        proxy_pass http://10.0.0.5:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

}
`
	if files[0].Content != wantContent {
		t.Errorf("content mismatch:\nwant:\n%s\ngot:\n%s", wantContent, files[0].Content)
	}
	if files[0].Mode != 0o644 {
		t.Errorf("mode: want 0644 got %o", files[0].Mode)
	}
}

func TestRender_DomainMultipleRoutesSorted(t *testing.T) {
	files, err := Render([]IngressCtx{{
		EdgeServerID: 1, FileStem: "x", MatchKind: MatchKindDomain, Domain: "x.com",
		Routes: []RouteCtx{
			{Sort: 2, Path: "/b", UpstreamURL: "http://1:1"},
			{Sort: 1, Path: "/a", UpstreamURL: "http://2:2"},
			{Sort: 1, Path: "/c", UpstreamURL: "http://3:3"},
		},
	}})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	c := files[0].Content
	ia := strings.Index(c, "/a")
	ic := strings.Index(c, "/c")
	ib := strings.Index(c, "/b")
	if !(ia < ic && ic < ib) {
		t.Errorf("route order broken: /a@%d /c@%d /b@%d", ia, ic, ib)
	}
}

func TestRender_PathMode(t *testing.T) {
	files, err := Render([]IngressCtx{{
		EdgeServerID: 1, FileStem: "appA", MatchKind: MatchKindPath, Domain: "shared.com",
		Routes: []RouteCtx{{Path: "/api", UpstreamURL: "http://1.2.3.4:80"}},
	}})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("path 模式应输出 location 文件 + hub 站点，want 2 got %d", len(files))
	}
	// 排序后 hub (sites-available/serverhub-app-hub) 在 app-locations/appA.conf 之前
	if files[0].Path != "/etc/nginx/app-locations/appA.conf" {
		t.Errorf("path[0]: %q", files[0].Path)
	}
	if files[1].Path != "/etc/nginx/sites-available/serverhub-app-hub" {
		t.Errorf("path[1]: %q", files[1].Path)
	}
	wantLoc := `location /api {
    proxy_pass http://1.2.3.4:80;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}

`
	if files[0].Content != wantLoc {
		t.Errorf("path location content:\nwant:\n%s\ngot:\n%s", wantLoc, files[0].Content)
	}
	wantHub := `server {
    listen 80;
    server_name _;

    include /etc/nginx/app-locations/*.conf;
}`
	if files[1].Content != wantHub {
		t.Errorf("hub content mismatch:\nwant:\n%s\ngot:\n%s", wantHub, files[1].Content)
	}
}

func TestRender_PathMultipleApps_OneHub(t *testing.T) {
	files, err := Render([]IngressCtx{
		{EdgeServerID: 1, FileStem: "a", MatchKind: MatchKindPath, Domain: "x.com", Routes: []RouteCtx{{Path: "/a", UpstreamURL: "http://1:1"}}},
		{EdgeServerID: 1, FileStem: "b", MatchKind: MatchKindPath, Domain: "x.com", Routes: []RouteCtx{{Path: "/b", UpstreamURL: "http://2:2"}}},
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	hub := 0
	for _, f := range files {
		if strings.HasSuffix(f.Path, HubSiteName) {
			hub++
		}
	}
	if hub != 1 {
		t.Errorf("hub 应只渲染一次，实际 %d 次", hub)
	}
	if len(files) != 3 {
		t.Errorf("want 3 files (2 location + 1 hub), got %d", len(files))
	}
}

func TestRender_WebSocketInjects(t *testing.T) {
	files, _ := Render([]IngressCtx{{
		EdgeServerID: 1, FileStem: "ws", MatchKind: MatchKindDomain, Domain: "ws.com",
		Routes: []RouteCtx{{Path: "/", UpstreamURL: "http://1:1", WebSocket: true}},
	}})
	c := files[0].Content
	for _, want := range []string{
		"proxy_http_version 1.1;",
		"proxy_set_header Upgrade $http_upgrade;",
		`proxy_set_header Connection "upgrade";`,
	} {
		if !strings.Contains(c, want) {
			t.Errorf("missing %q in:\n%s", want, c)
		}
	}
}

func TestRender_ExtraInjected(t *testing.T) {
	files, _ := Render([]IngressCtx{{
		EdgeServerID: 1, FileStem: "e", MatchKind: MatchKindPath, Domain: "x.com",
		Routes: []RouteCtx{{Path: "/", UpstreamURL: "http://1:1", Extra: "client_max_body_size 100m;"}},
	}})
	if !strings.Contains(files[0].Content, "    client_max_body_size 100m;\n") {
		t.Errorf("extra not injected with proper indent:\n%s", files[0].Content)
	}
}

func TestRender_EmptyRoutesSkipped(t *testing.T) {
	files, err := Render([]IngressCtx{{
		EdgeServerID: 1, FileStem: "empty", MatchKind: MatchKindDomain, Domain: "x.com",
		Routes: nil,
	}})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("空 routes 应跳过；got %d files", len(files))
	}
}

func TestRender_UnknownMatchKindErrors(t *testing.T) {
	_, err := Render([]IngressCtx{{
		EdgeServerID: 1, FileStem: "x", MatchKind: "weird", Domain: "x.com",
		Routes: []RouteCtx{{Path: "/", UpstreamURL: "http://1:1"}},
	}})
	if err == nil {
		t.Fatal("expected error on unknown match_kind")
	}
}

func TestRender_DomainModeRequiresDomain(t *testing.T) {
	_, err := Render([]IngressCtx{{
		EdgeServerID: 1, FileStem: "x", MatchKind: MatchKindDomain, Domain: "",
		Routes: []RouteCtx{{Path: "/", UpstreamURL: "http://1:1"}},
	}})
	if err == nil {
		t.Fatal("domain 模式无 domain 应报错")
	}
}

func TestRender_StemRequired(t *testing.T) {
	_, err := Render([]IngressCtx{{
		EdgeServerID: 1, FileStem: "", MatchKind: MatchKindPath, Domain: "x.com",
		Routes: []RouteCtx{{Path: "/", UpstreamURL: "http://1:1"}},
	}})
	if err == nil {
		t.Fatal("空 FileStem 应报错")
	}
}

func TestRender_OutputSortedByPath(t *testing.T) {
	files, _ := Render([]IngressCtx{
		{EdgeServerID: 1, FileStem: "z", MatchKind: MatchKindDomain, Domain: "z.com", Routes: []RouteCtx{{Path: "/", UpstreamURL: "http://1:1"}}},
		{EdgeServerID: 1, FileStem: "a", MatchKind: MatchKindDomain, Domain: "a.com", Routes: []RouteCtx{{Path: "/", UpstreamURL: "http://1:1"}}},
	})
	if len(files) != 2 {
		t.Fatalf("want 2 got %d", len(files))
	}
	if !(files[0].Path < files[1].Path) {
		t.Errorf("output not sorted: %s, %s", files[0].Path, files[1].Path)
	}
}

func TestRender_ByteEquivalentToOldApplyPath(t *testing.T) {
	// 与旧 applyPath 的字节级对照（来自 backend/api/approutes/handler.go 312-331）。
	files, _ := Render([]IngressCtx{{
		EdgeServerID: 1, FileStem: "demo", MatchKind: MatchKindPath, Domain: "_",
		Routes: []RouteCtx{
			{Sort: 0, Path: "/api", UpstreamURL: "http://127.0.0.1:9000"},
			{Sort: 1, Path: "/admin", UpstreamURL: "http://127.0.0.1:9001", Extra: "client_max_body_size 50m;"},
		},
	}})
	want := "location /api {\n" +
		"    proxy_pass http://127.0.0.1:9000;\n" +
		"    proxy_set_header Host $host;\n" +
		"    proxy_set_header X-Real-IP $remote_addr;\n" +
		"    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n" +
		"    proxy_set_header X-Forwarded-Proto $scheme;\n" +
		"}\n\n" +
		"location /admin {\n" +
		"    proxy_pass http://127.0.0.1:9001;\n" +
		"    proxy_set_header Host $host;\n" +
		"    proxy_set_header X-Real-IP $remote_addr;\n" +
		"    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n" +
		"    proxy_set_header X-Forwarded-Proto $scheme;\n" +
		"    client_max_body_size 50m;\n" +
		"}\n\n"
	// files[0] 是 app-locations/demo.conf（按 Path 排序在 hub 之前）
	if files[0].Content != want {
		t.Errorf("byte mismatch with old applyPath:\nwant:\n%q\ngot:\n%q", want, files[0].Content)
	}
}

func TestRender_ByteEquivalentToOldApplySite(t *testing.T) {
	// 与旧 applySite 的字节级对照（来自 backend/api/approutes/handler.go 362-384）。
	files, _ := Render([]IngressCtx{{
		EdgeServerID: 1, FileStem: "demo", MatchKind: MatchKindDomain, Domain: "demo.example.com",
		Routes: []RouteCtx{
			{Sort: 0, Path: "/", UpstreamURL: "http://127.0.0.1:9000"},
		},
	}})
	want := "server {\n" +
		"    listen 80;\n" +
		"    server_name demo.example.com;\n\n" +
		"    location / {\n" +
		"        proxy_pass http://127.0.0.1:9000;\n" +
		"        proxy_set_header Host $host;\n" +
		"        proxy_set_header X-Real-IP $remote_addr;\n" +
		"        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n" +
		"        proxy_set_header X-Forwarded-Proto $scheme;\n" +
		"    }\n\n" +
		"}\n"
	if files[0].Content != want {
		t.Errorf("byte mismatch with old applySite:\nwant:\n%q\ngot:\n%q", want, files[0].Content)
	}
}

func TestRenderHubSite(t *testing.T) {
	f, err := RenderHubSite()
	if err != nil {
		t.Fatal(err)
	}
	if f.Path != "/etc/nginx/sites-available/serverhub-app-hub" {
		t.Errorf("hub path: %q", f.Path)
	}
	if !strings.Contains(f.Content, "include /etc/nginx/app-locations/*.conf;") {
		t.Errorf("hub content missing include: %s", f.Content)
	}
}
