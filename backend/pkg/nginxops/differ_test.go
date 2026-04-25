package nginxops

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/serverhub/serverhub/pkg/nginxrender"
)

func hashHex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func TestParseInspect_BasicAndEmpty(t *testing.T) {
	if got, _ := ParseInspect(""); len(got) != 0 {
		t.Errorf("空输入应返回空 map，got %d entries", len(got))
	}

	body1 := "hello"
	body2 := "world"
	in := strings.Join([]string{
		"/etc/nginx/sites-available/foo-sh.conf\t" + hashHex(body1) + "\t" + base64.StdEncoding.EncodeToString([]byte(body1)),
		"/etc/nginx/app-locations/bar.conf\t" + hashHex(body2) + "\t" + base64.StdEncoding.EncodeToString([]byte(body2)),
	}, "\n") + "\n"
	got, err := ParseInspect(in)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 entries, got %d", len(got))
	}
	if got["/etc/nginx/sites-available/foo-sh.conf"].Content != body1 {
		t.Errorf("foo content mismatch")
	}
	if got["/etc/nginx/app-locations/bar.conf"].Hash != hashHex(body2) {
		t.Errorf("bar hash mismatch")
	}
}

func TestParseInspect_BadLineErrs(t *testing.T) {
	if _, err := ParseInspect("only-one-field\n"); err == nil {
		t.Fatal("应在格式异常时返回错误")
	}
	if _, err := ParseInspect("/p\thash\tnotbase64!!!\n"); err == nil {
		t.Fatal("应在 base64 解码失败时返回错误")
	}
}

func TestDiff_PureAdd(t *testing.T) {
	desired := []nginxrender.ConfigFile{{Path: "/x", Content: "hi"}}
	changes := Diff(desired, map[string]ActualFile{})
	if len(changes) != 1 || changes[0].Kind != ChangeAdd || changes[0].Path != "/x" {
		t.Fatalf("want single add /x, got %+v", changes)
	}
	if changes[0].NewHash != hashHex("hi") {
		t.Errorf("hash mismatch")
	}
}

func TestDiff_PureDelete(t *testing.T) {
	actual := map[string]ActualFile{"/y": {Path: "/y", Content: "bye", Hash: hashHex("bye")}}
	changes := Diff(nil, actual)
	if len(changes) != 1 || changes[0].Kind != ChangeDelete || changes[0].OldContent != "bye" {
		t.Fatalf("want single delete /y, got %+v", changes)
	}
}

func TestDiff_Update(t *testing.T) {
	desired := []nginxrender.ConfigFile{{Path: "/z", Content: "new"}}
	actual := map[string]ActualFile{"/z": {Path: "/z", Content: "old", Hash: hashHex("old")}}
	changes := Diff(desired, actual)
	if len(changes) != 1 || changes[0].Kind != ChangeUpdate {
		t.Fatalf("want update, got %+v", changes)
	}
	if changes[0].OldContent != "old" || changes[0].NewContent != "new" {
		t.Errorf("content fields wrong: %+v", changes[0])
	}
}

func TestDiff_NoOp(t *testing.T) {
	desired := []nginxrender.ConfigFile{{Path: "/a", Content: "same"}}
	actual := map[string]ActualFile{"/a": {Path: "/a", Content: "same", Hash: hashHex("same")}}
	changes := Diff(desired, actual)
	if len(changes) != 0 {
		t.Errorf("等内容应返回空集，got %+v", changes)
	}
}

func TestDiff_Mixed_OutputSortedByPath(t *testing.T) {
	desired := []nginxrender.ConfigFile{
		{Path: "/c", Content: "newC"},
		{Path: "/a", Content: "addA"},
	}
	actual := map[string]ActualFile{
		"/c": {Path: "/c", Content: "oldC", Hash: hashHex("oldC")},
		"/b": {Path: "/b", Content: "delB", Hash: hashHex("delB")},
	}
	changes := Diff(desired, actual)

	// expect: add /a, delete /b, update /c — already sorted by Path
	paths := []string{}
	kinds := []ChangeKind{}
	for _, c := range changes {
		paths = append(paths, c.Path)
		kinds = append(kinds, c.Kind)
	}
	wantPaths := []string{"/a", "/b", "/c"}
	wantKinds := []ChangeKind{ChangeAdd, ChangeDelete, ChangeUpdate}
	if !reflect.DeepEqual(paths, wantPaths) {
		t.Errorf("paths order: %v want %v", paths, wantPaths)
	}
	if !reflect.DeepEqual(kinds, wantKinds) {
		t.Errorf("kinds order: %v want %v", kinds, wantKinds)
	}
	// 通用稳定性：再 sort 一次结果不变
	cp := make([]Change, len(changes))
	copy(cp, changes)
	sort.Slice(cp, func(i, j int) bool { return cp[i].Path < cp[j].Path })
	if !reflect.DeepEqual(cp, changes) {
		t.Errorf("Diff 输出不是按 path 升序")
	}
}

func TestSnapshot_BuildsExpectedCommand(t *testing.T) {
	r := newFakeRunner()
	r.defaults = ""
	path, err := Snapshot(r, 42)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(path, "/var/lib/serverhub/nginx-bak/42-") {
		t.Errorf("path: %q", path)
	}
	if len(r.calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(r.calls))
	}
	cmd := r.calls[0]
	for _, want := range []string{
		"sudo -n mkdir -p '/var/lib/serverhub/nginx-bak'",
		"sudo -n tar -C /etc/nginx -czf",
		"-mtime +7 -delete",
	} {
		if !strings.Contains(cmd, want) {
			t.Errorf("snapshot 命令缺少 %q：\n%s", want, cmd)
		}
	}
}

func TestRestore_RejectsEmptyPath(t *testing.T) {
	if err := Restore(newFakeRunner(), ""); err == nil {
		t.Fatal("空路径应报错")
	}
}

func TestAcquire_Mutex(t *testing.T) {
	rel := Acquire(99)
	done := make(chan struct{})
	go func() {
		rel2 := Acquire(99)
		rel2()
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("第二次 Acquire 应被首次阻塞")
	default:
	}
	rel()
	<-done // 释放后应解除阻塞
}

func TestFormatChangeset(t *testing.T) {
	// 自给自足：避免直接依赖 internal helper hash truncation 写错偏移
	addHash := hashHex("a")
	updNew, updOld := hashHex("new"), hashHex("old")
	cs := []Change{
		{Kind: ChangeAdd, Path: "/p1", NewHash: addHash},
		{Kind: ChangeUpdate, Path: "/p2", OldHash: updOld, NewHash: updNew},
		{Kind: ChangeDelete, Path: "/p3"},
	}
	got := formatChangeset(cs)
	for _, want := range []string{
		"+ /p1 (" + addHash[:8] + ")",
		"~ /p2 (" + updOld[:8] + " → " + updNew[:8] + ")",
		"- /p3",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("formatChangeset 缺少 %q：\n%s", want, got)
		}
	}
	if formatChangeset(nil) != "" {
		t.Errorf("空集应返回空串")
	}
}

func TestSanitizeStem(t *testing.T) {
	cases := map[string]string{
		"":                "default",
		"_":               "default",
		"app.example.com": "app_example_com",
		"foo bar":         "foo-bar",
		"a/b":             "a-b",
		"x_y-z.0":         "x_y-z_0",
	}
	for in, want := range cases {
		if got := sanitizeStem(in); got != want {
			t.Errorf("sanitizeStem(%q)=%q want %q", in, got, want)
		}
	}
}
