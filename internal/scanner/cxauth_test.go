package scanner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCxAuth_APIKeyMode(t *testing.T) {
	t.Setenv("CX1_APIKEY", "the-api-key")
	cfg := &Config{CxAPIKeyEnv: "CX1_APIKEY"}
	if CxAuthModeOf(cfg) != CxAuthAPIKey {
		t.Fatalf("expected apikey mode")
	}
	if err := CxAuthAvailable(cfg); err != nil {
		t.Fatalf("apikey should be available: %v", err)
	}
	args, env, keys, err := CxAuth(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 0 {
		t.Errorf("apikey mode should add no auth args, got %v", args)
	}
	if strings.Join(env, " ") != "CX_APIKEY=the-api-key" || len(keys) != 1 || keys[0] != "CX_APIKEY" {
		t.Errorf("expected CX_APIKEY env injection, got env=%v keys=%v", env, keys)
	}
}

func TestCxAuth_APIKeyFallbackToCX_APIKEY(t *testing.T) {
	t.Setenv("CX1_APIKEY", "")
	t.Setenv("CX_APIKEY", "fallback-key")
	cfg := &Config{CxAPIKeyEnv: "CX1_APIKEY"}
	if err := CxAuthAvailable(cfg); err != nil {
		t.Errorf("should fall back to CX_APIKEY: %v", err)
	}
}

func TestCxAuth_APIKeyMissing(t *testing.T) {
	t.Setenv("CX1_APIKEY", "")
	t.Setenv("CX_APIKEY", "")
	cfg := &Config{CxAPIKeyEnv: "CX1_APIKEY"}
	if err := CxAuthAvailable(cfg); err == nil {
		t.Errorf("expected error when no API key set")
	}
}

func TestCxAuth_ClientCredentialsMode(t *testing.T) {
	t.Setenv("CX_CLIENT_SECRET", "shhh-secret")
	cfg := &Config{
		CxClientID:    "my-oauth-client",
		CxBaseURI:     "https://example.ast.checkmarx.net",
		CxBaseAuthURI: "https://example.iam.checkmarx.net",
		CxTenant:      "example-tenant",
	}
	if CxAuthModeOf(cfg) != CxAuthClient {
		t.Fatalf("client id should select client-credentials mode")
	}
	if err := CxAuthAvailable(cfg); err != nil {
		t.Fatalf("client-credentials should be available: %v", err)
	}
	args, env, keys, err := CxAuth(cfg)
	if err != nil {
		t.Fatal(err)
	}
	line := strings.Join(args, " ")
	for _, want := range []string{
		"--client-id my-oauth-client",
		"--base-uri https://example.ast.checkmarx.net",
		"--base-auth-uri https://example.iam.checkmarx.net",
		"--tenant example-tenant",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("missing %q in args: %s", want, line)
		}
	}
	// Secret must be injected via env, never argv.
	if strings.Contains(line, "shhh-secret") || strings.Contains(line, "--client-secret") {
		t.Errorf("client secret leaked into argv: %s", line)
	}
	if strings.Join(env, " ") != "CX_CLIENT_SECRET=shhh-secret" || keys[0] != "CX_CLIENT_SECRET" {
		t.Errorf("expected CX_CLIENT_SECRET env injection, got env=%v keys=%v", env, keys)
	}
}

func TestCxAuth_ClientCredentialsMissingURIs(t *testing.T) {
	t.Setenv("CX_CLIENT_SECRET", "x")
	cfg := &Config{CxClientID: "id"} // no base-uri/auth-uri/tenant
	if err := CxAuthAvailable(cfg); err == nil {
		t.Errorf("expected error for client-credentials without URIs/tenant")
	}
}

func TestCxAuth_ClientSecretFromFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "secret")
	if err := os.WriteFile(p, []byte("file-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{
		CxClientID:         "id",
		CxBaseURI:          "https://a",
		CxBaseAuthURI:      "https://i",
		CxTenant:           "t",
		CxClientSecretFile: p,
	}
	_, env, _, err := CxAuth(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(env, " ") != "CX_CLIENT_SECRET=file-secret" {
		t.Errorf("secret should be read (trimmed) from file, got %v", env)
	}
}
