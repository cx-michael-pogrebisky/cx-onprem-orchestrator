# cx-onprem-orchestrator

A single CLI that orchestrates an arbitrary subset of Checkmarx scanners in one
invocation, replicates the **`cx` CLI threshold and file-filter syntax verbatim**,
lets you pass **exact native flags** to each underlying scanner, and reduces all
results to **one meaningful exit code**. Runs unmodified across **15 CI systems** (GitHub Actions, GitLab, Azure DevOps,
Jenkins, TeamCity, Bamboo, Bitbucket, Buildkite, CircleCI, AWS CodeBuild, Travis,
Drone, Semaphore, AppVeyor, Codefresh).

| Engine | `--scanners` token | Driver | Threshold |
|---|---|---|---|
| CxSAST on-prem | `sast` | CxConsolePlugin (Java 11+) | pass-through `-SAST*` caps |
| SCA (SCA Resolver) | `sca` | `cx scan create --sca-resolver` | pass-through `cx --threshold` |
| KICS (IaC) | `kics` | `kics` binary / `checkmarx/kics` (digest-pinned) | wrapper-side count |
| Secrets (2ms) | `secrets` | `2ms` binary / `checkmarx/2ms` (digest-pinned) | wrapper-side count |
| Container Security | `containers` | `cx scan create --scan-types container-security` | pass-through `cx --threshold` |
| DAST | `dast` | *(post-v1)* | — |

## Install — use the fat image (recommended)

**The fat image is the recommended way to run this tool.** It bundles the
orchestrator **plus every engine tool** (`cx`, `ScaResolver`, `kics`, `2ms`,
`CxConsolePlugin`, Java 21) — all **digest-pinned** — so the five engines work with
**no extra setup**:

> **Note:** SCA dependency resolution additionally needs the *scanned project's*
> language build tools (node/npm, python/pip, maven, …) present in the execution
> environment. These are **not** bundled — they vary per project, so provide them
> in your CI before scanning. See [SCA Resolver](docs/sca-resolver.md).

```bash
# Docker
docker run --rm -v "$PWD":/work -w /work \
  -e CX1_APIKEY -e CXSAST_URL -e CXSAST_USERNAME -e CXSAST_PASSWORD \
  ghcr.io/cx-michael-pogrebisky/cx-onprem-orchestrator:fat \
  run --scanners sast,sca,kics,secrets,containers --threshold "sast-critical=1;sca-high=5"

# Podman — drop-in (add :Z to the mount on SELinux hosts: RHEL/Fedora)
podman run --rm -v "$PWD":/work:Z -w /work \
  -e CX1_APIKEY -e CXSAST_URL -e CXSAST_USERNAME -e CXSAST_PASSWORD \
  ghcr.io/cx-michael-pogrebisky/cx-onprem-orchestrator:fat \
  run --scanners sast,sca,kics,secrets,containers --threshold "sast-critical=1;sca-high=5"
```

> Runs identically under **Docker or Podman**. Many orgs prefer Podman because
> Docker **Desktop** (the standard Docker path on Windows/macOS) needs a paid
> subscription at 250+ employees or >$10M revenue, while Podman is Apache-2.0/free —
> see [Container runtime](docs/ci.md#container-runtime--docker-or-podman). On
> **Windows**, run the **native binary** (no container needed) — see
> [Windows agents](docs/ci/windows.md).

<details>
<summary><b>Advanced (not recommended): slim image / standalone binary</b></summary>

You *can* instead grab the static binary or the slim image and install/version-manage
`cx`, `ScaResolver` (+`Configuration.yml`), `kics` (+queries), `2ms`,
`CxConsolePlugin`, and **Java 11+** on each agent yourself — but this is a
**significantly less recommended** path (more moving parts, version drift, lost digest
pinning). See [docs/ci.md → Image choice](docs/ci.md#image-choice).

```bash
curl -sSfL https://github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/releases/download/v1.0.0/cx-onprem-orchestrator_linux_amd64.tar.gz | tar xz
```
</details>

## Quickstart

Inside the fat image, tool paths are pre-set (`CXOO_SCA_RESOLVER`, `CXOO_SAST_PATH`,
KICS queries), so a run is just engines + threshold:

```bash
cx-onprem-orchestrator run \
  --scanners sast,sca,kics,secrets,containers \
  --threshold "sast-critical=1;sca-high=5;iac-security-low=10;secrets-total=1;containers-high=3" \
  --file-filter "!**/**,**/src/**" --use-gitignore
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
- **[docs/ci.md](docs/ci.md)** — ready-to-paste pipeline snippets for all 15 CI systems.

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

## License

[MIT](LICENSE) © Michael Pogrebisky.
