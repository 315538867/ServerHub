package discovery

import (
	"strings"

	"github.com/serverhub/serverhub/pkg/runner"
)

// secretKeyHints — substring match (case-insensitive) on the env key. When any
// hint matches, the value is flagged as Secret so the UI masks it by default.
var secretKeyHints = []string{
	"password", "passwd", "secret", "token", "apikey", "api_key",
	"private", "credential", "jdbc", "dsn", "auth", "session",
	"jwt", "encrypt", "salt", "cookie",
}

// keyLooksSecret returns true when the env key name suggests the value is
// sensitive. Conservative: false negatives are acceptable, false positives
// (over-masking) are also OK — the user can untoggle.
func keyLooksSecret(k string) bool {
	lower := strings.ToLower(k)
	// Plain "key" alone is too noisy (matches anything); require it as suffix.
	if strings.HasSuffix(lower, "_key") || strings.HasSuffix(lower, "key") && len(lower) > 3 {
		return true
	}
	for _, h := range secretKeyHints {
		if strings.Contains(lower, h) {
			return true
		}
	}
	return false
}

// parseKVPairs splits a KEY=VAL list into EnvKV entries, dropping empties and
// stripping surrounding quotes ("..."  or '...') from the value.
func parseKVPairs(pairs []string) []EnvKV {
	out := make([]EnvKV, 0, len(pairs))
	for _, p := range pairs {
		p = strings.TrimSpace(p)
		if p == "" || strings.HasPrefix(p, "#") {
			continue
		}
		eq := strings.IndexByte(p, '=')
		if eq <= 0 {
			continue
		}
		k := strings.TrimSpace(p[:eq])
		v := strings.TrimSpace(p[eq+1:])
		v = unquote(v)
		if k == "" {
			continue
		}
		out = append(out, EnvKV{Key: k, Value: v, Secret: keyLooksSecret(k)})
	}
	return out
}

func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// readEnvFile pulls a .env-style file via the runner and returns parsed pairs.
// Lines may be `KEY=VAL`, optionally with surrounding quotes; `# ...` comments
// and blanks are ignored. Errors (file missing) silently return an empty list —
// the caller's overall scan should not fail because of one missing env file.
func readEnvFile(rn runner.Runner, path string) []EnvKV {
	if path == "" {
		return nil
	}
	out, err := rn.Run("cat " + shellQuote(path) + " 2>/dev/null")
	if err != nil || strings.TrimSpace(out) == "" {
		return nil
	}
	return parseKVPairs(strings.Split(out, "\n"))
}

// mergeEnv appends `extra` onto `base`, skipping keys already present in base.
// Stable order: base entries keep their relative order, then any new ones from
// extra in their original order.
func mergeEnv(base, extra []EnvKV) []EnvKV {
	if len(extra) == 0 {
		return base
	}
	seen := make(map[string]struct{}, len(base))
	for _, kv := range base {
		seen[kv.Key] = struct{}{}
	}
	for _, kv := range extra {
		if _, ok := seen[kv.Key]; ok {
			continue
		}
		seen[kv.Key] = struct{}{}
		base = append(base, kv)
	}
	return base
}
