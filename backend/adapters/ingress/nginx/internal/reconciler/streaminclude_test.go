package reconciler

import (
	"strings"
	"testing"
)

const sampleNginxConf = `user www-data;
worker_processes auto;

events { worker_connections 768; }

http {
    include /etc/nginx/sites-enabled/*;
}
`

func TestEnsureStreamInclude_AddsBlockWhenWanted(t *testing.T) {
	fr := newFakeRunner()
	fr.onContains("sudo -n cat", sampleNginxConf, nil)
	// 写盘默认成功（fakeRunner 默认无错）

	ch, err := ensureStreamInclude(fr, true)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ch == nil {
		t.Fatal("应产生 Change，反馈给 rollback")
	}
	if !strings.Contains(ch.NewContent, streamMarkerBegin) ||
		!strings.Contains(ch.NewContent, streamMarkerEnd) ||
		!strings.Contains(ch.NewContent, "include /etc/nginx/streams.conf;") {
		t.Errorf("新内容缺 marker / include:\n%s", ch.NewContent)
	}
	if ch.OldContent != sampleNginxConf {
		t.Errorf("OldContent 应保留原文，便于 rollback")
	}
	// 应触发一次写盘
	hasWrite := false
	for _, c := range fr.calls {
		if strings.Contains(c, "tee") && strings.Contains(c, "nginx.conf") {
			hasWrite = true
			break
		}
	}
	if !hasWrite {
		t.Errorf("应该执行写 nginx.conf；calls=%v", fr.calls)
	}
}

func TestEnsureStreamInclude_NoOpWhenAlreadyPresent(t *testing.T) {
	already := sampleNginxConf + "\n" + streamMarkerBegin + "\ninclude /etc/nginx/streams.conf;\n" + streamMarkerEnd + "\n"
	fr := newFakeRunner()
	fr.onContains("sudo -n cat", already, nil)

	ch, err := ensureStreamInclude(fr, true)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ch != nil {
		t.Errorf("内容已对齐应 NoOp，实际产生 Change：%+v", ch)
	}
	for _, c := range fr.calls {
		if strings.Contains(c, "tee") {
			t.Errorf("NoOp 不应写盘：%s", c)
		}
	}
}

func TestEnsureStreamInclude_RemovesBlockWhenUnwanted(t *testing.T) {
	with := sampleNginxConf + "\n" + streamMarkerBegin + "\ninclude /etc/nginx/streams.conf;\n" + streamMarkerEnd + "\n"
	fr := newFakeRunner()
	fr.onContains("sudo -n cat", with, nil)

	ch, err := ensureStreamInclude(fr, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ch == nil {
		t.Fatal("有 marker 块时 want=false 应触发改写")
	}
	if strings.Contains(ch.NewContent, streamMarkerBegin) {
		t.Errorf("removal 后不应残留 marker:\n%s", ch.NewContent)
	}
}

func TestEnsureStreamInclude_NoOpWhenAbsentAndUnwanted(t *testing.T) {
	fr := newFakeRunner()
	fr.onContains("sudo -n cat", sampleNginxConf, nil)

	ch, err := ensureStreamInclude(fr, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ch != nil {
		t.Errorf("无 marker + want=false 应 NoOp，实际：%+v", ch)
	}
}

func TestStripStreamMarkers_Idempotent(t *testing.T) {
	in := "head\n" + streamMarkerBegin + "\ninclude x;\n" + streamMarkerEnd + "\nfoot\n"
	out := stripStreamMarkers(in)
	if strings.Contains(out, streamMarkerBegin) {
		t.Errorf("应剥光 marker:\n%s", out)
	}
	if !strings.Contains(out, "head") || !strings.Contains(out, "foot") {
		t.Errorf("不应误伤上下文:\n%s", out)
	}
	// 再 strip 一次结果不变
	if stripStreamMarkers(out) != out {
		t.Errorf("strip 不是幂等的")
	}
}

func TestAppendStreamMarkers_NoTrailingBlankPileUp(t *testing.T) {
	in := "x\n\n\n"
	out := appendStreamMarkers(in, "/etc/nginx/streams.conf")
	if strings.Contains(out, "\n\n\n\n") {
		t.Errorf("不应堆积空行:\n%q", out)
	}
	if !strings.HasSuffix(out, streamMarkerEnd+"\n") {
		t.Errorf("应以 marker_end + \\n 收尾:\n%q", out)
	}
}
