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

// ── gRPC（P2）────────────────────────────────────────────────────────────────

func TestRender_GRPC_DomainAddsHTTP2(t *testing.T) {
	files, err := Render([]IngressCtx{{
		EdgeServerID: 1, FileStem: "g", MatchKind: MatchKindDomain, Domain: "g.com",
		Routes: []RouteCtx{{
			Path: "/", Protocol: "grpc", UpstreamURL: "http://10.0.0.5:9000",
		}},
	}})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	c := files[0].Content
	if !strings.Contains(c, "listen 80 http2;") {
		t.Errorf("grpc 应触发 http2 listen, got:\n%s", c)
	}
	if !strings.Contains(c, "grpc_pass grpc://10.0.0.5:9000;") {
		t.Errorf("应生成 grpc_pass 而不是 proxy_pass:\n%s", c)
	}
	if strings.Contains(c, "proxy_pass") {
		t.Errorf("grpc 路由不应有 proxy_pass:\n%s", c)
	}
	if !strings.Contains(c, "grpc_set_header Host $host;") {
		t.Errorf("缺 grpc_set_header Host:\n%s", c)
	}
	// X-Forwarded-* 是 HTTP/1 习惯，gRPC 路由不应注入
	if strings.Contains(c, "X-Forwarded-For") || strings.Contains(c, "X-Forwarded-Proto") {
		t.Errorf("grpc 路由不应注入 X-Forwarded-* 头:\n%s", c)
	}
}

func TestRender_GRPC_HTTPSUpgrade(t *testing.T) {
	files, _ := Render([]IngressCtx{{
		EdgeServerID: 1, FileStem: "gs", MatchKind: MatchKindDomain, Domain: "gs.com",
		Routes: []RouteCtx{{Path: "/", Protocol: "grpc", UpstreamURL: "https://up:443"}},
	}})
	if !strings.Contains(files[0].Content, "grpc_pass grpcs://up:443;") {
		t.Errorf("https 上游应转 grpcs://, got:\n%s", files[0].Content)
	}
}

func TestRender_GRPC_BareUpstream(t *testing.T) {
	files, _ := Render([]IngressCtx{{
		EdgeServerID: 1, FileStem: "gb", MatchKind: MatchKindDomain, Domain: "gb.com",
		Routes: []RouteCtx{{Path: "/", Protocol: "grpc", UpstreamURL: "10.0.0.7:9000"}},
	}})
	if !strings.Contains(files[0].Content, "grpc_pass grpc://10.0.0.7:9000;") {
		t.Errorf("裸 host:port 应自动补 grpc:// 前缀, got:\n%s", files[0].Content)
	}
}

func TestRender_GRPC_AlreadyPrefixed(t *testing.T) {
	files, _ := Render([]IngressCtx{{
		EdgeServerID: 1, FileStem: "gp", MatchKind: MatchKindDomain, Domain: "gp.com",
		Routes: []RouteCtx{{Path: "/", Protocol: "grpc", UpstreamURL: "grpc://up:9000"}},
	}})
	if !strings.Contains(files[0].Content, "grpc_pass grpc://up:9000;") {
		t.Errorf("已经是 grpc:// 不应重复加前缀, got:\n%s", files[0].Content)
	}
}

func TestRender_HTTPListenUnchangedWithoutGRPC(t *testing.T) {
	files, _ := Render([]IngressCtx{{
		EdgeServerID: 1, FileStem: "h", MatchKind: MatchKindDomain, Domain: "h.com",
		Routes: []RouteCtx{{Path: "/", UpstreamURL: "http://up:80"}},
	}})
	if !strings.Contains(files[0].Content, "listen 80;") || strings.Contains(files[0].Content, "http2") {
		t.Errorf("纯 HTTP 不应带 http2:\n%s", files[0].Content)
	}
}

func TestRender_MixedHTTPAndGRPC_BothInSameServerWithHTTP2(t *testing.T) {
	files, _ := Render([]IngressCtx{{
		EdgeServerID: 1, FileStem: "m", MatchKind: MatchKindDomain, Domain: "m.com",
		Routes: []RouteCtx{
			{Path: "/api", UpstreamURL: "http://up:1"},
			{Path: "/rpc", Protocol: "grpc", UpstreamURL: "http://up:2"},
		},
	}})
	c := files[0].Content
	if !strings.Contains(c, "listen 80 http2;") {
		t.Errorf("含 grpc 即使有 http 路由也要 http2, got:\n%s", c)
	}
	if !strings.Contains(c, "proxy_pass http://up:1;") {
		t.Errorf("http 路由仍走 proxy_pass:\n%s", c)
	}
	if !strings.Contains(c, "grpc_pass grpc://up:2;") {
		t.Errorf("grpc 路由走 grpc_pass:\n%s", c)
	}
}

func TestRender_GRPC_PathMode(t *testing.T) {
	files, _ := Render([]IngressCtx{{
		EdgeServerID: 1, FileStem: "p", MatchKind: MatchKindPath, Domain: "shared.com",
		Routes: []RouteCtx{{Path: "/grpc", Protocol: "grpc", UpstreamURL: "http://up:9"}},
	}})
	// path 模式只输出 location 列表 + hub 站点，没有 listen 行
	var loc *ConfigFile
	for i := range files {
		if strings.Contains(files[i].Path, "app-locations") {
			loc = &files[i]
			break
		}
	}
	if loc == nil {
		t.Fatalf("没找到 app-locations 文件: %+v", files)
	}
	if !strings.Contains(loc.Content, "grpc_pass grpc://up:9;") {
		t.Errorf("path 模式 grpc 应渲染 grpc_pass:\n%s", loc.Content)
	}
}

func TestGRPCURL_Helpers(t *testing.T) {
	cases := map[string]string{
		"http://h:1":  "grpc://h:1",
		"https://h:2": "grpcs://h:2",
		"grpc://h:3":  "grpc://h:3",
		"grpcs://h:4": "grpcs://h:4",
		"h:5":         "grpc://h:5",
		"":            "grpc://",
	}
	for in, want := range cases {
		if got := grpcURL(in); got != want {
			t.Errorf("grpcURL(%q)=%q want %q", in, got, want)
		}
	}
}

// ── stream（P2-D3）────────────────────────────────────────────────────────────

func TestRender_StreamTCP_Single(t *testing.T) {
	files, err := Render([]IngressCtx{{
		EdgeServerID: 1, FileStem: "db", MatchKind: MatchKindDomain, Domain: "_",
		Routes: []RouteCtx{{
			Path: "/", Protocol: "tcp", ListenPort: 5432,
			UpstreamURL: "http://10.0.0.5:5432",
		}},
	}})
	if err != nil {
		t.Fatalf("render err: %v", err)
	}
	if len(files) != 1 || files[0].Path != StreamsConf {
		t.Fatalf("stream-only ingress 应只输出 streams.conf, got %+v", files)
	}
	want := "stream {\n" +
		"    server {\n" +
		"        listen 5432;\n" +
		"        proxy_pass 10.0.0.5:5432;\n" +
		"    }\n" +
		"}\n"
	if files[0].Content != want {
		t.Errorf("streams.conf mismatch:\nwant:\n%s\ngot:\n%s", want, files[0].Content)
	}
}

func TestRender_StreamUDP_AppendsUDPMarker(t *testing.T) {
	files, _ := Render([]IngressCtx{{
		EdgeServerID: 1, FileStem: "dns", MatchKind: MatchKindDomain, Domain: "_",
		Routes: []RouteCtx{{
			Path: "/", Protocol: "udp", ListenPort: 53,
			UpstreamURL: "10.0.0.5:53",
		}},
	}})
	if !strings.Contains(files[0].Content, "listen 53 udp;") {
		t.Errorf("udp 路由 listen 行应带 udp:\n%s", files[0].Content)
	}
	if !strings.Contains(files[0].Content, "proxy_pass 10.0.0.5:53;") {
		t.Errorf("proxy_pass 应剥掉 scheme:\n%s", files[0].Content)
	}
}

func TestRender_StreamAggregatesAcrossIngresses(t *testing.T) {
	files, _ := Render([]IngressCtx{
		{EdgeServerID: 1, FileStem: "a", MatchKind: MatchKindDomain, Domain: "_",
			Routes: []RouteCtx{{Path: "/", Protocol: "tcp", ListenPort: 9000, UpstreamURL: "http://up:1"}}},
		{EdgeServerID: 1, FileStem: "b", MatchKind: MatchKindDomain, Domain: "_",
			Routes: []RouteCtx{{Path: "/", Protocol: "udp", ListenPort: 53, UpstreamURL: "up:2"}}},
	})
	streamCnt := 0
	for _, f := range files {
		if f.Path == StreamsConf {
			streamCnt++
		}
	}
	if streamCnt != 1 {
		t.Fatalf("跨 ingress 的 stream 路由应聚合到单个 streams.conf, got %d", streamCnt)
	}
	c := files[0].Content
	// listen 端口升序：53 在 9000 之前
	i53 := strings.Index(c, "listen 53")
	i9k := strings.Index(c, "listen 9000")
	if !(i53 > 0 && i53 < i9k) {
		t.Errorf("stream server 应按 listen_port 升序:\n%s", c)
	}
}

func TestRender_StreamMixedWithHTTPSameIngress(t *testing.T) {
	files, _ := Render([]IngressCtx{{
		EdgeServerID: 1, FileStem: "m", MatchKind: MatchKindDomain, Domain: "m.com",
		Routes: []RouteCtx{
			{Path: "/api", UpstreamURL: "http://up:1"},
			{Path: "/", Protocol: "tcp", ListenPort: 5432, UpstreamURL: "http://up:5432"},
		},
	}})
	var hasSite, hasStream bool
	for _, f := range files {
		switch {
		case strings.HasSuffix(f.Path, "-sh.conf"):
			hasSite = true
			if strings.Contains(f.Content, "5432") {
				t.Errorf("http server 块不应混入 stream 内容:\n%s", f.Content)
			}
		case f.Path == StreamsConf:
			hasStream = true
		}
	}
	if !hasSite || !hasStream {
		t.Errorf("混合路由应同时输出 site + streams.conf, got files=%+v", files)
	}
}

func TestRender_StreamMissingListenPortErrors(t *testing.T) {
	_, err := Render([]IngressCtx{{
		EdgeServerID: 1, FileStem: "x", MatchKind: MatchKindDomain, Domain: "_",
		Routes: []RouteCtx{{Path: "/", Protocol: "tcp", UpstreamURL: "h:1"}},
	}})
	if err == nil {
		t.Fatal("缺 ListenPort 应报错")
	}
}

func TestRender_StreamOnlyIngressDoesNotCreateSite(t *testing.T) {
	files, _ := Render([]IngressCtx{{
		EdgeServerID: 1, FileStem: "x", MatchKind: MatchKindDomain, Domain: "_",
		Routes: []RouteCtx{{Path: "/", Protocol: "tcp", ListenPort: 1234, UpstreamURL: "h:1"}},
	}})
	for _, f := range files {
		if strings.HasSuffix(f.Path, "-sh.conf") {
			t.Errorf("纯 stream ingress 不应输出 -sh.conf: %s", f.Path)
		}
	}
}

func TestRender_StreamExtraInjected(t *testing.T) {
	files, _ := Render([]IngressCtx{{
		EdgeServerID: 1, FileStem: "e", MatchKind: MatchKindDomain, Domain: "_",
		Routes: []RouteCtx{{
			Path: "/", Protocol: "tcp", ListenPort: 6379, UpstreamURL: "h:6379",
			Extra: "proxy_timeout 1h;",
		}},
	}})
	if !strings.Contains(files[0].Content, "        proxy_timeout 1h;\n") {
		t.Errorf("extra 应以 server 内缩进注入:\n%s", files[0].Content)
	}
}

func TestStreamUpstream_Helpers(t *testing.T) {
	cases := map[string]string{
		"http://h:1":  "h:1",
		"https://h:2": "h:2",
		"h:3":         "h:3",
		"":            "",
	}
	for in, want := range cases {
		if got := streamUpstream(in); got != want {
			t.Errorf("streamUpstream(%q)=%q want %q", in, got, want)
		}
	}
}

// ── TLS / HTTPS（P2-D4）──────────────────────────────────────────────────────

func TestRender_TLS_DomainAddsHTTPSServerWithCertLines(t *testing.T) {
	files, err := Render([]IngressCtx{{
		EdgeServerID: 1, FileStem: "s", MatchKind: MatchKindDomain, Domain: "s.com",
		TLSCertPath: "/etc/ssl/certs/s.pem",
		TLSKeyPath:  "/etc/ssl/private/s.key",
		Routes: []RouteCtx{{Path: "/", UpstreamURL: "http://up:1"}},
	}})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	c := files[0].Content
	// 80 段保留（未 force_https）
	if !strings.Contains(c, "listen 80;") {
		t.Errorf("默认 force_https=false 应保留 80 server:\n%s", c)
	}
	// 443 段
	if !strings.Contains(c, "listen 443 ssl;") {
		t.Errorf("应渲染 listen 443 ssl;\n%s", c)
	}
	if !strings.Contains(c, "ssl_certificate /etc/ssl/certs/s.pem;") {
		t.Errorf("缺 ssl_certificate:\n%s", c)
	}
	if !strings.Contains(c, "ssl_certificate_key /etc/ssl/private/s.key;") {
		t.Errorf("缺 ssl_certificate_key:\n%s", c)
	}
	// 80 与 443 都应该挂 location（双轨）
	if strings.Count(c, "location /") < 2 {
		t.Errorf("80/443 应同时挂同一组 routes:\n%s", c)
	}
}

func TestRender_TLS_ForceHTTPSEmitsRedirect(t *testing.T) {
	files, _ := Render([]IngressCtx{{
		EdgeServerID: 1, FileStem: "f", MatchKind: MatchKindDomain, Domain: "f.com",
		TLSCertPath: "/c.pem", TLSKeyPath: "/c.key",
		ForceHTTPS:  true,
		Routes:      []RouteCtx{{Path: "/", UpstreamURL: "http://up:1"}},
	}})
	c := files[0].Content
	if !strings.Contains(c, "return 301 https://$host$request_uri;") {
		t.Errorf("force_https 应触发 80→443 跳转:\n%s", c)
	}
	// 80 server 体里不应有 location（被 redirect 占据）
	idx80 := strings.Index(c, "listen 80;")
	idx443 := strings.Index(c, "listen 443 ssl;")
	if idx80 < 0 || idx443 < 0 || idx80 >= idx443 {
		t.Fatalf("80 段应在 443 段之前:\n%s", c)
	}
	// 80~443 之间不应有 location 关键字
	if strings.Contains(c[idx80:idx443], "location ") {
		t.Errorf("force_https 80 段不应挂 location:\n%s", c[idx80:idx443])
	}
	// 443 段应有 location
	if !strings.Contains(c[idx443:], "location /") {
		t.Errorf("443 段应挂 location:\n%s", c[idx443:])
	}
}

func TestRender_TLS_GRPCUpgrades443ToHTTP2(t *testing.T) {
	files, _ := Render([]IngressCtx{{
		EdgeServerID: 1, FileStem: "g", MatchKind: MatchKindDomain, Domain: "g.com",
		TLSCertPath: "/c.pem", TLSKeyPath: "/c.key",
		Routes:      []RouteCtx{{Path: "/", Protocol: "grpc", UpstreamURL: "http://up:9"}},
	}})
	c := files[0].Content
	if !strings.Contains(c, "listen 443 ssl http2;") {
		t.Errorf("grpc + tls 应是 listen 443 ssl http2:\n%s", c)
	}
	if !strings.Contains(c, "listen 80 http2;") {
		t.Errorf("grpc 即使加了 tls,80 段也仍是 http2:\n%s", c)
	}
}

func TestRender_TLS_NoCertDoesNothing(t *testing.T) {
	files, _ := Render([]IngressCtx{{
		EdgeServerID: 1, FileStem: "n", MatchKind: MatchKindDomain, Domain: "n.com",
		Routes: []RouteCtx{{Path: "/", UpstreamURL: "http://up:1"}},
	}})
	c := files[0].Content
	if strings.Contains(c, "listen 443") || strings.Contains(c, "ssl_certificate") {
		t.Errorf("无 cert 不应渲染 443/ssl_*:\n%s", c)
	}
}

func TestRender_TLS_MissingKeyErrors(t *testing.T) {
	_, err := Render([]IngressCtx{{
		EdgeServerID: 1, FileStem: "x", MatchKind: MatchKindDomain, Domain: "x.com",
		TLSCertPath: "/c.pem", // KeyPath 留空
		Routes:      []RouteCtx{{Path: "/", UpstreamURL: "http://up:1"}},
	}})
	if err == nil {
		t.Fatal("仅给 TLSCertPath 缺 TLSKeyPath 应报错")
	}
}

func TestRender_TLS_ByteAlignedNonTLSCase(t *testing.T) {
	// TLS 字段为零值时,字节级输出与 P1 旧版完全一致——保护 reconciler 不因升级触发全量重写。
	files, _ := Render([]IngressCtx{{
		EdgeServerID: 1, FileStem: "demo", MatchKind: MatchKindDomain, Domain: "demo.example.com",
		Routes: []RouteCtx{{Sort: 0, Path: "/", UpstreamURL: "http://127.0.0.1:9000"}},
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
		t.Errorf("非 TLS 路径字节回归:\nwant:\n%q\ngot:\n%q", want, files[0].Content)
	}
}
