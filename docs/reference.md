# CLI & environment reference

Complete reference for every command, flag, and environment variable. Flags fall
into three tiers: **A** (unified, cx-identical), **B** (raw per-engine passthrough),
**C** (selection / context / auth / output / orchestration). Generate this live
with `cx-onprem-orchestrator run --help`.

## Commands

| Command | Purpose |
|---|---|
| `run` | Run the selected scanners and gate on thresholds. |
| `validate` | Resolve + validate config and print the exact native argv per engine **without scanning** (alias: `run --dry-run`). |
| `detect` | Print the detected CI system and resolved branch/commit/repo/workspace. |
| `version` | Print the tool version and platform. |
| `completion` | Generate a shell completion script (bash/zsh/fish/powershell). |

## Tier C — selection & context

| Flag | Default | Description |
|---|---|---|
| `--scanners <list>` | **required** | The analog of `cx --scan-types`. Engines to run: `sast,sca,kics,secrets,containers` (or the explicit `all`). **Only the listed engines run.** Unlike `cx`, there is **no default** — omitting `--scanners` is a config error (exit 30); the tool never implicitly runs "all". Aliases: `iac`/`iac-security`→kics, `2ms`/`twoms`→secrets, `container`/`container-security`→containers. |
| `-s, --source <dir>` | CI workspace, else `.` | Path to the code under test. |
| `--project-name <name>` | derived from repo/source | Project name reported to the backends (default for every engine). |
| `--sast-project-name <name>` | `--project-name` | Override the project name for **CxSAST** only (the CxSAST project becomes `<team>\<name>`). |
| `--cx-project-name <name>` | `--project-name` | Override the project name for the **Cx1** engines (SCA + Container Security) only. |
| `--branch <name>` | CI-detected | Branch name. |

## Tier A — threshold

| Flag | Default | Description |
|---|---|---|
| `--threshold "<engine>-<sev>=<n>;…"` | none | cx-identical gate; inclusive `>=`, integer `>= 1`, `;`/`,` interchangeable, case-insensitive. Engines: `sast`, `sca`, `iac-security` (alias `kics`), `containers`, plus `secrets-total` and (post-v1) `dast-<sev>`. Severities: `critical,high,medium,low,info`. |

## Tier A — filters (cx-verbatim; types preserved)

| Flag | Type | Engine | Native target |
|---|---|---|---|
| `-f, --file-filter <glob>` | glob/Nant | all source engines | global include/exclude |
| `-i, --file-include <exts>` | csv | cx engines | extra extensions |
| `--use-gitignore` | bool | cx engines | honor `.gitignore` |
| `--sast-filter <glob>` | glob (lossy) | SAST | `-LocationPathExclude`/`-LocationFilesExclude` (names) |
| `--sca-filter <regex>` | **regex** | SCA | `cx --sca-filter` |
| `--iac-security-filter <glob>` (alias `--kics-filter`) | glob | KICS | `kics --exclude-paths` |
| `--containers-file-folder-filter <glob>` | glob | Containers | `cx --containers-file-folder-filter` |
| `--containers-package-filter <regex>` | **regex** | Containers | `cx --containers-package-filter` |
| `--containers-image-tag-filter <wildcard>` | **wildcard** | Containers | `cx --containers-image-tag-filter` |
| `--secrets-filter <name-glob>` | name glob | Secrets | `2ms --ignore-pattern` |

## Tier A — output

| Flag | Default | Description |
|---|---|---|
| `--report-formats <csv>` | `json,sarif` | Formats to emit per engine (see [reports.md](reports.md)). |
| `--<engine>-report-formats <csv>` | inherits `--report-formats` | Per-engine override of the format set for one engine only, e.g. `--sca-report-formats json` to skip the slow SCA SARIF export. |
| `--output-path <dir>` | `./cxoo-reports` | Root directory for collected reports + `run-summary.json`. |
| `--output-name <prefix>` | `cxoo` | Summary metadata base name. |
| `--ignore-on-exit <mode>` | `none` | `none|results|errors|all` — unified mapping onto kics/2ms/dast exit suppression. |

## Tier C — orchestration

| Flag | Default | Description |
|---|---|---|
| `--on-missing <mode>` | `fail` | `fail` (exit 31) or `skip-warn` when a requested engine's tool/auth is missing. |
| `--conflict <mode>` | `error` | `error` / `raw-wins` / `unified-wins` for Tier-A vs Tier-B flag collisions. |
| `--async` | false | Run async (rejected with `--threshold`; gating needs synchronous scans). |
| `--parallel <N>` | 0 | Run up to N engines concurrently (0 = sequential). |
| `--fail-fast` | false | Stop after the first engine execution failure. |
| `--timeout <dur>` | 0 | Overall timeout, e.g. `30m`. |
| `--dry-run` | false | (run) print resolved native argv per engine and exit without scanning. |

## Tier C — Cx1 auth (SCA, Containers)

See [authentication.md](authentication.md). API key is default; supplying
`--cx-client-id` selects client-credentials and requires the URIs + tenant.

| Flag | Env default | Description |
|---|---|---|
| `--cx-apikey-env <NAME>` | `CX1_APIKEY`→`CX_APIKEY` | env var holding the API key |
| `--cx-client-id <id>` | `CX_CLIENT_ID` | OAuth2 client ID (selects client-credentials) |
| `--cx-client-secret-env <NAME>` | `CX_CLIENT_SECRET` | env var holding the client secret |
| `--cx-client-secret-file <PATH>` | — | `0600` file holding the client secret |
| `--cx-base-uri <url>` | `CX_BASE_URI` | AST system URI (required for client-credentials) |
| `--cx-base-auth-uri <url>` | `CX_BASE_AUTH_URI` | IAM/auth URI (required for client-credentials) |
| `--cx-tenant <name>` | `CX_TENANT` | tenant (required for client-credentials) |

## Tier C — CxSAST auth (SAST)

| Flag | Env default | Description |
|---|---|---|
| `--sast-server <url>` | `CXSAST_URL` | `-CxServer` |
| `--sast-user-env <NAME>` | `CXSAST_USERNAME` | `-CxUser` |
| `--sast-password-env <NAME>` | `CXSAST_PASSWORD` | `-CxPassword` |
| `--sast-token-env <NAME>` | — | `-CxToken` (preferred) |
| `--sast-sso` | — | `-useSSO` |
| `--sast-java <path>` | `$JAVA_HOME`/`java` | Java 11+ runtime (JDK home or a `java` binary). On Windows, pass the full path to `java.exe`, or set `JAVA_HOME` (the `.exe` suffix is added automatically). See [ci/windows.md](ci/windows.md). |
| `--sast-team <path>` | — | CxSAST team/full-path prefix for the project, e.g. `CxServer/SP` → `-ProjectName CxServer\SP\<project>` (CxSAST rejects a bare project name). Forward slashes are normalized to backslashes. |

## Tier C — tool resolution (per engine)

> These flags are mainly for the **non-fat-image** (self-managed tools) path, which
> is [not recommended](ci.md#image-choice). The fat image resolves everything for you.

For each engine token `E ∈ {sast, sca, kics, secrets, containers}`:

| Flag | Description |
|---|---|
| `--E-mode <native\|docker>` | force resolution mode (default: native if on PATH, else docker; SAST native-only; cx engines native-only) |
| `--E-path <path>` | binary/JAR/script path (native) |
| `--E-image <ref>` | docker image (digest-pinned via `manifest.lock` by default) |
| `--sca-resolver <path>` | ScaResolver executable. **SCA always runs in Resolver mode** — required (or `CXOO_SCA_RESOLVER` in the fat image), else SCA fails with exit 31. |
| `--sca-resolver-arg=<tok>` | raw arg forwarded to the **ScaResolver** binary via `cx --sca-resolver-params` (repeatable, `=`-bound). Distinct from `--sca-arg`, which targets the **cx** command. See [sca-resolver.md](sca-resolver.md). |
| `--container-images <csv>` | images for the containers scan |

## Tier B — raw passthrough

For each engine token `E`: `--E-arg=<one native token>` — repeatable, `=`-bound,
forwarded verbatim **after** the wrapper-translated args. Examples:
`--sast-arg=-ReportPDF=report.pdf`, `--kics-arg=--exclude-categories=Encryption`,
`--sca-resolver-arg=--excludes --sca-resolver-arg='**/test/**'`.

## Environment variables

| Variable | Read by | Purpose |
|---|---|---|
| `CX1_APIKEY` | SCA, Containers | Cx1 API key (default; re-exported to `cx` as `CX_APIKEY`) |
| `CX_APIKEY` | SCA, Containers | fallback API key name (and what `cx` ultimately reads) |
| `CX_CLIENT_ID` | SCA, Containers | OAuth2 client ID (alt to `--cx-client-id`) |
| `CX_CLIENT_SECRET` | SCA, Containers | OAuth2 client secret (default name) |
| `CX_BASE_URI` / `CX_BASE_AUTH_URI` / `CX_TENANT` | SCA, Containers | client-credentials endpoints (alt to flags) |
| `CXSAST_URL` | SAST | `-CxServer` default |
| `CXSAST_USERNAME` / `CXSAST_PASSWORD` | SAST | `-CxUser` / `-CxPassword` default names |
| `JAVA_HOME` | SAST | Java 11+ runtime discovery |
| `CXOO_SCA_RESOLVER` | SCA | default `--sca-resolver` path (set in the fat image) |
| `CXOO_SAST_PATH` | SAST | default `--sast-path` (set in the fat image) |
| `CXOO_KICS_QUERIES_PATH` | KICS | native-mode query assets path (set in the fat image) |
| `CXSCAN_BRANCH` / `CXSCAN_COMMIT` / `CXSCAN_REPO` / `CXSCAN_WORKSPACE` | CI detect | explicit context overrides; honored for TeamCity and for any **undetected** CI (e.g. Google Cloud Build, Woodpecker) |
| CI-provider vars | CI detect | branch/commit/repo/workspace for 15 auto-detected systems — see [ci.md](ci.md) |

## Exit codes

`0` success · `10` threshold breach · `11` partial · `20`/`21` engine failure (+breach)
· `30` config error · `31` prerequisite missing · `40` orchestration error · `130`
interrupted. A lossless `run-summary.json` is always written.
