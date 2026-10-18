package redact

import (
	"encoding/json"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

var patterns = []*regexp.Regexp{
	regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?(?:-----END [A-Z ]*PRIVATE KEY-----|$)`),
	regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/=-]+`),
	regexp.MustCompile(`\b(?:sk-(?:proj-)?[A-Za-z0-9_-]{12,}|gh[pousr]_[A-Za-z0-9_]{16,}|github_pat_[A-Za-z0-9_]{16,}|AKIA[A-Z0-9]{16})\b`),
	regexp.MustCompile(`(?im)["']?\b[A-Z0-9_]*(?:API[_-]?KEY|TOKEN|PASSWORD|PASSWD|SECRET)[A-Z0-9_]*["']?\s*[:=]\s*(?:"[^"\r\n]*"|'[^'\r\n]*'|[^\s\n,}]+)`),
	regexp.MustCompile(`(?i)--(?:api-key|token|password|secret)(?:=|\s+)(?:"[^"\r\n]*"|'[^'\r\n]*'|[^\s]+)`),
	regexp.MustCompile(`(?i)\bBasic\s+[A-Za-z0-9+/=]{8,}`),
	regexp.MustCompile(`https?://[^\s<>"']+`),
}

// JSON preserves structure while stripping known secret fields and text patterns.
func JSON(raw json.RawMessage) json.RawMessage {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return json.RawMessage(`{}`)
	}
	var walk func(any) any
	walk = func(v any) any {
		switch x := v.(type) {
		case string:
			return Text(x)
		case []any:
			for i := range x {
				x[i] = walk(x[i])
			}
		case map[string]any:
			safe := make(map[string]any, len(x))
			for k, child := range x {
				lower := strings.ToLower(k)
				if strings.Contains(lower, "token") || strings.Contains(lower, "password") || strings.Contains(lower, "secret") || strings.Contains(lower, "api_key") || lower == "authorization" {
					safe[Text(k)] = "[REDACTED]"
				} else {
					safe[Text(k)] = walk(child)
				}
			}
			return safe
		}
		return v
	}
	b, _ := json.Marshal(walk(value))
	return b
}

func Text(s string) string {
	for i, p := range patterns {
		if i == len(patterns)-1 {
			s = p.ReplaceAllStringFunc(s, func(raw string) string {
				u, e := url.Parse(raw)
				if e != nil {
					return "[REDACTED URL]"
				}
				if u.User != nil {
					u.User = url.User("REDACTED")
				}
				q := u.Query()
				for k := range q {
					l := strings.ToLower(k)
					if strings.Contains(l, "token") || strings.Contains(l, "key") || strings.Contains(l, "secret") || strings.Contains(l, "password") {
						q.Set(k, "REDACTED")
					}
				}
				u.RawQuery = q.Encode()
				return u.String()
			})
		} else {
			s = p.ReplaceAllString(s, "[REDACTED]")
		}
	}
	return s
}
func SecretPath(path string) bool {
	p := strings.ToLower(filepath.ToSlash(path))
	for _, part := range strings.Split(p, "/") {
		if part == ".env" || strings.HasPrefix(part, ".env.") || part == ".ssh" || part == ".aws" || part == ".npmrc" || part == ".netrc" || part == "credentials" || strings.HasSuffix(part, ".pem") || strings.HasSuffix(part, ".key") || strings.HasPrefix(part, "id_rsa") || strings.HasPrefix(part, "id_ed25519") {
			return true
		}
	}
	return false
}
