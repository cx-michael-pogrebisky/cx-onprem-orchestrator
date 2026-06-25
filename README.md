# cx-onprem-orchestrator

A single CLI that orchestrates an arbitrary subset of Checkmarx scanners in one
invocation, replicates the **`cx` CLI threshold and file-filter syntax verbatim**,
lets you pass **exact native flags** to each underlying scanner, and reduces all
results to **one meaningful exit code**. Runs unmodified across Jenkins, TeamCity,
Bamboo, GitHub Actions, GitLab, Azure DevOps, and Bitbucket.

| Engine | `--scanners` token | Driver | Threshold |
|---|---|---|---|
| CxSAST on-prem | `sast` | CxConsolePlugin (Java 11+) | pass-through `-SAST*` caps |
| SCA (SCA Resolver) | `sca` | `cx scan create --sca-resolver` | pass-through `cx --threshold` |
| KICS (IaC) | `kics` | `kics` binary / `checkmarx/kics` (digest-pinned) | wrapper-side count |
| Secrets (2ms) | `secrets` | `2ms` binary / `checkmarx/2ms` (digest-pinned) | wrapper-side count |
| Container Security | `containers` | `cx scan create --scan-types container-security` | pass-through `cx --threshold` |
| DAST | `dast` | *(post-v1)* | — |

## Install

```bash
# Static binary (Linux/macOS/Windows × amd64/arm64) from GitHub Releases:
curl -sSfL https://github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/releases/download/vX.Y.Z/cx-onprem-orchestrator_linux_amd64.tar.gz | tar xz
# Or the batteries-included fat image (bundles cx, ScaResolver, kics, 2ms, CxConsolePlugin, Java):
docker run --rm -v "$PWD":/work ghcr.io/cx-michael-pogrebisky/cx-onprem-orchestrator:fat run --scanners kics ...
```

## Quickstart

```bash
cx-onprem-orchestrator run \
  --scanners sast,sca,kics,secrets,containers \
  --threshold "sast-critical=1;sca-high=5;iac-security-low=10;secrets-total=1;containers-high=3" \
  --file-filter "!**/**,**/src/**" --use-gitignore \
  --sca-resolver /opt/sca/ScaResolver \
  --sast-path /opt/cxconsole/runCxConsole.sh --sast-java "$JAVA_HOME_11"
```

Use `validate` (alias `run --dry-run`) to print the exact native argv per engine
without scanning, and `detect` to print the auto-detected CI context.

## Threshold syntax (identical to `cx`)

```
--threshold "<engine>-<severity>=<limit>;..."
```
- Separators `;` and `,` are interchangeable; whitespace ignored; case-insensitive.
- Breach is **inclusive** (`count >= limit`); `limit` must be an integer `>= 1`.
- Engines: `sast`, `sca`, `iac-security` (alias `kics`), `containers`, plus the
  extension `secrets-total` (2ms) and `dast-<sev>` (post-v1).
- Severities: `critical, high, medium, low, info`.

## File filters (identical to `cx`)

Global `--file-filter` / `--file-include` / `--use-gitignore`; per-engine
`--sast-filter` (glob, lossy→CxSAST names), `--sca-filter` (regex),
`--iac-security-filter` (glob), `--containers-{file-folder,package,image-tag}-filter`,
`--secrets-filter`. Glob/Nant: leading `!` excludes; a list starting with `!`
includes-all-then-filters.

## Pass exact native flags

```
--<engine>-arg=<one native token>      # repeatable, =-bound, forwarded verbatim
--sast-arg=-ReportPDF=report.pdf
--sca-resolver-arg=--excludes --sca-resolver-arg='**/generated/**'
```

## Authentication (secrets via env, never argv)

The tool reads the *name* of an env var (or a `0600` file) holding each secret and
injects it into the child process — secrets never appear in argv, logs, or `--dry-run`.

| Env var (default) | Used by | Mapped to |
|---|---|---|
| `CX1_APIKEY` | SCA, containers | re-exported as `CX_APIKEY` (auto-derives base-uri/tenant) |
| `CX_CLIENT_ID` / `CX_CLIENT_SECRET` | SCA, containers | Cx1 **OAuth2 client-credentials** (alt to API key; needs `--cx-base-uri`/`--cx-base-auth-uri`/`--cx-tenant`) |
| `CXSAST_URL` | CxSAST | `-CxServer` |
| `CXSAST_USERNAME` / `CXSAST_PASSWORD` | CxSAST | `-CxUser` / `-CxPassword` |

Cx1 supports two modes — **API key** (default) or **OAuth2 client-credentials**.
Full details and all use cases: **[docs/authentication.md](docs/authentication.md)**.

## Documentation

- **[docs/sca-resolver.md](docs/sca-resolver.md)** — SCA is always run via SCA Resolver; how to pass ScaResolver vs cx arguments (`--sca-resolver-arg` vs `--sca-arg`), with examples.
- **[docs/authentication.md](docs/authentication.md)** — Cx1 API key & client-credentials, CxSAST auth, all use cases.
- **[docs/reports.md](docs/reports.md)** — artifacts per scanner, the format-support matrix, and `--report-formats` behavior.
- **[docs/reference.md](docs/reference.md)** — every command, flag, and environment variable.
- **[docs/ci.md](docs/ci.md)** — ready-to-paste pipeline snippets for all 7 CI systems.

## Exit codes (stable contract)

`0` pass · `10` threshold breach · `20`/`21` engine failure (+breach) ·
`30` config error · `31` prerequisite missing · `40` orchestration error · `130` interrupted.

A lossless `run-summary.json` is always written to `--output-path`.

## Prerequisites

- **CxSAST**: a **Java 11+** JRE on PATH (or `--sast-java`) and the CxConsolePlugin
  (`--sast-path`). The CLI's classes are Java 8, but its bundled JGit 6.10 is Java 11.
- **SCA**: the `cx` CLI + the `ScaResolver` binary (`--sca-resolver`) with a sibling
  `Configuration.yml`, plus the scanned project's package managers.
- **KICS / 2ms**: a `kics`/`2ms` binary, or Docker (images pulled by pinned digest).
- **Containers / SCA**: a Cx1 API key.

See [docs/ci.md](docs/ci.md) for ready-to-paste pipeline snippets for all 7 CI systems.
