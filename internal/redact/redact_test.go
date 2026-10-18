package redact

import (
	"encoding/json"
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

func TestQuotedCredentialsAndStructuredMetadata(t *testing.T) {
	raw := `{"api_key":"synthetic-json-value","message":"password='synthetic spaced value'","nested":{"Authorization":"Basic c3ludGhldGljOnRlc3Q="},"safe":"diagnostic remains"}`
	out := JSON(json.RawMessage(raw))
	if !json.Valid(out) || strings.Contains(string(out), "synthetic") || !strings.Contains(string(out), "diagnostic remains") {
		t.Fatal(string(out))
	}
	for _, input := range []string{raw, `--token "synthetic flag value"`, `PASSWORD='synthetic spaced value'`} {
		if strings.Contains(Text(input), "synthetic") {
			t.Fatal("quoted credential leaked")
		}
	}
}
