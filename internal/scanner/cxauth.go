package scanner

import (
	"fmt"
	"os"
	"strings"
)

// CxAuthMode is the resolved Cx1 authentication mode.
type CxAuthMode string

const (
	CxAuthAPIKey CxAuthMode = "apikey"             // --apikey / CX_APIKEY (auto-derives base/tenant)
	CxAuthClient CxAuthMode = "client-credentials" // --client-id/--client-secret + base-uri/base-auth-uri/tenant
)

// DefaultClientSecretEnv is the env var the OAuth2 client secret is read from when
// --cx-client-secret-env is not given. It matches cx's own CX_CLIENT_SECRET.
const DefaultClientSecretEnv = "CX_CLIENT_SECRET"

// cxClientID resolves the client id from the explicit flag or the CX_CLIENT_ID env.
func cxClientID(cfg *Config) string {
	if cfg.CxClientID != "" {
		return cfg.CxClientID
	}
	return os.Getenv("CX_CLIENT_ID")
}

// CxAuthModeOf reports which Cx1 auth mode the config selects. Presence of a
// client id (flag or CX_CLIENT_ID) selects client-credentials; otherwise API key.
func CxAuthModeOf(cfg *Config) CxAuthMode {
	if cxClientID(cfg) != "" {
		return CxAuthClient
	}
	return CxAuthAPIKey
}

// clientSecretValue reads the OAuth2 client secret from --cx-client-secret-file or
// the configured env var (default CX_CLIENT_SECRET).
func clientSecretValue(cfg *Config) (string, error) {
	if cfg.CxClientSecretFile != "" {
		b, err := os.ReadFile(cfg.CxClientSecretFile)
		if err != nil {
			return "", fmt.Errorf("reading --cx-client-secret-file: %w", err)
		}
		return strings.TrimSpace(string(b)), nil
	}
	envName := cfg.CxClientSecretEnv
	if envName == "" {
		envName = DefaultClientSecretEnv
	}
	return os.Getenv(envName), nil
}

// apiKeyValue reads the Cx1 API key from the configured env (default CX1_APIKEY),
// falling back to CX_APIKEY.
func apiKeyValue(cfg *Config) string {
	if cfg.CxAPIKeyEnv != "" {
		if v := os.Getenv(cfg.CxAPIKeyEnv); v != "" {
			return v
		}
	}
	return os.Getenv("CX_APIKEY")
}

// CxAuthAvailable validates that the selected Cx1 auth mode has everything it
// needs, WITHOUT performing IO beyond reading env/files. Used by Available().
func CxAuthAvailable(cfg *Config) error {
	switch CxAuthModeOf(cfg) {
	case CxAuthClient:
		var missing []string
		if cfg.CxBaseURI == "" {
			missing = append(missing, "--cx-base-uri")
		}
		if cfg.CxBaseAuthURI == "" {
			missing = append(missing, "--cx-base-auth-uri")
		}
		if cfg.CxTenant == "" {
			missing = append(missing, "--cx-tenant")
		}
		secret, err := clientSecretValue(cfg)
		if err != nil {
			return err
		}
		if secret == "" {
			missing = append(missing, "client secret (set --cx-client-secret-env / --cx-client-secret-file)")
		}
		if len(missing) > 0 {
			return fmt.Errorf("Cx1 client-credentials auth requires: %s", strings.Join(missing, ", "))
		}
	default: // API key
		if apiKeyValue(cfg) == "" {
			name := cfg.CxAPIKeyEnv
			if name == "" {
				name = "CX1_APIKEY"
			}
			return fmt.Errorf("Cx1 API key not set: env %s is empty (or switch to client-credentials with --cx-client-id)", name)
		}
	}
	return nil
}

// CxAuth resolves Cx1 auth into the cx CLI args and child env to inject. Secret
// values (API key, client secret) are injected via env, never placed on argv; the
// client id and the non-secret URIs/tenant are passed as flags so --dry-run shows
// them. EnvKeys lists the injected env-var NAMES for redacted --dry-run display.
func CxAuth(cfg *Config) (args []string, env []string, envKeys []string, err error) {
	switch CxAuthModeOf(cfg) {
	case CxAuthClient:
		secret, err := clientSecretValue(cfg)
		if err != nil {
			return nil, nil, nil, err
		}
		args = append(args,
			"--client-id", cxClientID(cfg),
			"--base-uri", cfg.CxBaseURI,
			"--base-auth-uri", cfg.CxBaseAuthURI,
			"--tenant", cfg.CxTenant,
		)
		if secret != "" {
			env = append(env, "CX_CLIENT_SECRET="+secret)
			envKeys = append(envKeys, "CX_CLIENT_SECRET")
		}
	default: // API key
		if v := apiKeyValue(cfg); v != "" {
			env = append(env, "CX_APIKEY="+v)
			envKeys = append(envKeys, "CX_APIKEY")
		}
		// The API key auto-derives base/auth/tenant, but honor explicit overrides.
		if cfg.CxBaseURI != "" {
			args = append(args, "--base-uri", cfg.CxBaseURI)
		}
		if cfg.CxBaseAuthURI != "" {
			args = append(args, "--base-auth-uri", cfg.CxBaseAuthURI)
		}
		if cfg.CxTenant != "" {
			args = append(args, "--tenant", cfg.CxTenant)
		}
	}
	return args, env, envKeys, nil
}
