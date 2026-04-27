package docker

import (
	"testing"

	"github.com/serverhub/serverhub/core/source"
)

// TestFingerprintByteCompatV1 锁定 v1 → v2 平移的字节级一致性。
// v1 算法: sha1("docker|<image>|<workdir>|<sortedBinds>|<sortedPorts>")
// 任何对算法的"轻微改动"(比如换 separator、改大小写、加版本号)都会让
// 已落库的 source_fingerprint 失配,把"已接管"按钮重新点亮——这是用户
// 不可见但破坏性极强的回归。本测用一组手算的预期值守门。
func TestFingerprintByteCompatV1(t *testing.T) {
	cases := []struct {
		name string
		c    source.Candidate
		want string // 用 `echo -n "key" | sha1sum` 离线手算得出
	}{
		{
			name: "minimal docker",
			c: source.Candidate{
				Kind: Kind,
				Suggested: source.SuggestedFields{
					Image: "nginx:1.25",
				},
			},
			// printf '%s' "docker|nginx:1.25|||" | shasum -a 1
			want: "d1adfd64a01291b481108536aad95cf791fbde9d",
		},
		{
			name: "binds 排序去空稳定",
			c: source.Candidate{
				Kind: Kind,
				Suggested: source.SuggestedFields{
					Image:   "redis:7",
					Workdir: "/data",
				},
				Raw: map[string]string{
					// 故意乱序 + 空白 + trailing 空,验证 normalizeList
					"binds": " /b:/data ,, /a:/etc ",
					"ports": "6379:6379",
				},
			},
			// 排序后:"/a:/etc,/b:/data" + ports "6379:6379"
			// printf '%s' "docker|redis:7|/data|/a:/etc,/b:/data|6379:6379" | shasum -a 1
			want: "9e5a03d1a7460ead7520c708cadb7d1e2ba05d5a",
		},
	}
	s := Scanner{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := s.Fingerprint(tc.c)
			if got != tc.want {
				t.Errorf("fingerprint drift: got %s, want %s", got, tc.want)
			}
		})
	}
}

// TestFingerprintStable 同一 candidate 多次调用必返同值(纯函数契约)。
func TestFingerprintStable(t *testing.T) {
	s := Scanner{}
	c := source.Candidate{
		Kind: Kind,
		Suggested: source.SuggestedFields{
			Image:   "alpine",
			Workdir: "/x",
		},
		Raw: map[string]string{"binds": "/a:/x", "ports": "80:80"},
	}
	first := s.Fingerprint(c)
	for i := 0; i < 10; i++ {
		if got := s.Fingerprint(c); got != first {
			t.Fatalf("Fingerprint 漂移: 第 %d 次 %s != %s", i, got, first)
		}
	}
}

func TestKindRegistered(t *testing.T) {
	if got := (Scanner{}).Kind(); got != "docker" {
		t.Errorf("Kind=%q, want docker", got)
	}
	// init() 已自注册,Default.Get 应能拿到。
	got, err := source.Default.Get("docker")
	if err != nil {
		t.Fatalf("source.Default.Get(docker) failed: %v", err)
	}
	if got.Kind() != "docker" {
		t.Errorf("registered scanner Kind=%q", got.Kind())
	}
}
