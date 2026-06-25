# Authentication

`cx-onprem-orchestrator` never accepts secret **values** on the command line — it
takes the **name of an environment variable** (or a `0600` file) that holds the
secret, reads it at run time, and injects it into the child process's environment.
Secrets therefore never appear in argv, the process list, `--dry-run` output, or
logs (`--dry-run` shows the env-var *names*, redacted).

There are two backends with independent auth:

- **Cx1 (CheckmarxOne)** — used by **SCA** and **Container Security**. Two
  mutually-exclusive modes: **API key** (default) or **OAuth2 client-credentials**.
- **CxSAST on-prem** — used by **SAST** (the CxConsolePlugin).

KICS and 2ms need no authentication.

---

## Cx1 — Mode A: API key (default)

A Cx1 API key is an offline token that **auto-derives** the base URI, IAM URI, and
tenant, so you only supply the key.

```bash
export CX1_APIKEY="eyJhbG..."        # the default env var the tool reads
cx-onprem-orchestrator run --scanners sca,containers --threshold "sca-high=5"
```

| Flag | Default | Meaning |
|---|---|---|
| `--cx-apikey-env <NAME>` | `CX1_APIKEY` (falls back to `CX_APIKEY`) | env var holding the API key |

The value is re-exported to the `cx` child as `CX_APIKEY`. You can point at a
differently-named variable: `--cx-apikey-env MY_CX_KEY`.

**Use cases**
- *Local / shell:* `export CX1_APIKEY=…` then run.
- *CI:* store the key as a masked secret named `CX1_APIKEY` (or your name + `--cx-apikey-env`).
- *Docker:* `docker run -e CX1_APIKEY … ghcr.io/.../cx-onprem-orchestrator:fat run …`.

---

## Cx1 — Mode B: OAuth2 client-credentials

Supplying `--cx-client-id` (or `CX_CLIENT_ID`) **selects client-credentials mode**.
Because there is no API key to derive from, you must also provide the base URI, the
IAM/auth URI, and the tenant. The orchestrator validates this up front (exit `30`
if any are missing).

```bash
export CX_CLIENT_SECRET="…"          # secret read from this env (default name)
cx-onprem-orchestrator run --scanners sca,containers \
  --cx-client-id    my-pipeline-client \
  --cx-base-uri     https://<region>.ast.checkmarx.net \
  --cx-base-auth-uri https://<region>.iam.checkmarx.net \
  --cx-tenant       <tenant> \
  --threshold "sca-high=5"
```

| Flag | Env (default) | Required | Meaning |
|---|---|---|---|
| `--cx-client-id <id>` | `CX_CLIENT_ID` | yes (selects this mode) | OAuth2 client ID (not secret) |
| `--cx-client-secret-env <NAME>` | reads `CX_CLIENT_SECRET` | one of env/file | env var holding the client secret |
| `--cx-client-secret-file <PATH>` | — | one of env/file | `0600` file holding the client secret |
| `--cx-base-uri <url>` | `CX_BASE_URI` | yes | AST system URI, e.g. `https://example.ast.checkmarx.net` |
| `--cx-base-auth-uri <url>` | `CX_BASE_AUTH_URI` | yes | IAM/auth URI, e.g. `https://example.iam.checkmarx.net` |
| `--cx-tenant <name>` | `CX_TENANT` | yes | Cx1 tenant/realm |

The client secret is injected as `CX_CLIENT_SECRET`; `--client-id`, `--base-uri`,
`--base-auth-uri`, `--tenant` are passed as (non-secret) flags to `cx`.

**Creating the OAuth client** (Cx1 → *Settings → Identity and Access Management →
OAuth Clients*): create a confidential client with the **client-credentials** grant
and assign it the **`ast-scanner`** role (least privilege for running scans). Copy
the generated secret into your CI secret store.

**Use cases**
- *CI (recommended):* set `CX_CLIENT_SECRET` as a masked variable; pass
  `--cx-client-id/--cx-base-uri/--cx-base-auth-uri/--cx-tenant` as job config.
- *File-based:* mount the secret as a `0600` file and use `--cx-client-secret-file`.

---

## CxSAST on-prem (SAST)

The CxConsolePlugin authenticates with the CxSAST server. The orchestrator reads
the server URL and credentials by env-var name (defaults shown). **Prefer a token
or SSO over a password.** The CxConsolePlugin has no env-based auth of its own, so
the orchestrator reads the secret from the named env var and passes it on the
child's argv — visible in that host's process list for the scan duration only.

| Flag | Default | Meaning |
|---|---|---|
| `--sast-server <url>` | `$CXSAST_URL` | `-CxServer` |
| `--sast-user-env <NAME>` | `CXSAST_USERNAME` | `-CxUser` |
| `--sast-password-env <NAME>` | `CXSAST_PASSWORD` | `-CxPassword` |
| `--sast-token-env <NAME>` | — | `-CxToken` (preferred over password) |
| `--sast-sso` | — | `-useSSO` (Windows SSO; preferred where available) |
| `--sast-java <path>` | `$JAVA_HOME` / `java` | Java **11+** runtime for the plugin |

```bash
export CXSAST_URL=http://cxsast.internal
export CXSAST_USERNAME=svc-checkmarx
export CXSAST_PASSWORD=…
cx-onprem-orchestrator run --scanners sast --threshold "sast-high=10" \
  --sast-path /opt/cxconsole/runCxConsole.sh
```

> CxSAST requires **Java 11+** (the CxConsolePlugin's bundled JGit 6.10 is Java-11
> bytecode). See the [README](../README.md#prerequisites).

---

## Summary of secret handling

| Secret | Read from | Injected to child as | On argv? |
|---|---|---|---|
| Cx1 API key | `CX1_APIKEY` / `--cx-apikey-env` | `CX_APIKEY` | never |
| Cx1 client secret | `CX_CLIENT_SECRET` / `--cx-client-secret-env` / `--cx-client-secret-file` | `CX_CLIENT_SECRET` | never |
| CxSAST password/token | `CXSAST_PASSWORD` / `CXSAST_USERNAME` / `--sast-token-env` | (CxConsolePlugin argv) | yes — plugin has no env auth |

Run `cx-onprem-orchestrator validate …` (or `run --dry-run`) to see the exact
resolved invocation per engine with all secret values redacted.
