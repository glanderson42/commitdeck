package redact

import (
	"strings"
	"testing"
)

func TestRedaction(t *testing.T) {
	raw := "API_KEY=supersecret\nAuthorization: Bearer top.secret.token\nhttps://user:password@example.com/repo?token=hidden\n-----BEGIN PRIVATE KEY-----\nprivatebytes\n-----END PRIVATE KEY-----"
	out := Text(raw)
	for _, secret := range []string{"supersecret", "top.secret.token", "user:password", "hidden", "privatebytes"} {
		if strings.Contains(out, secret) {
			t.Fatal("leaked", secret, out)
		}
	}
	if !SecretPath("a/.env.local") || !SecretPath(".aws/credentials") || SecretPath("src/retry.go") {
		t.Fatal("secret path detection")
	}
}
