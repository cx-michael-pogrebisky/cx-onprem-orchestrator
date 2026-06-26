# CI integration

`cx-onprem-orchestrator` auto-detects the CI system and extracts branch / commit /
repo / workspace, so you usually only pass `--scanners` and `--threshold`.

## Image choice

> ### ✅ Recommended: the **fat image**
> `ghcr.io/cx-michael-pogrebisky/cx-onprem-orchestrator:fat`
>
> The fat image is the **strongly recommended** way to run in CI. It bundles the
> orchestrator **plus every engine tool** — `cx`, `ScaResolver` (+ `Configuration.yml`),
> `kics` (+ its query assets), `2ms`, the `CxConsolePlugin`, and a **Java 21** runtime —
> all **pinned by sha256 digest**. Use it as your job's container image (or `docker run`
> it with the workspace mounted) and all five engines work with **no extra setup**.
>
> Why it's recommended:
> - **Nothing to install** — no per-agent provisioning of five heterogeneous tools.
> - **Reproducible & supply-chain-pinned** — every tool is a known, digest-locked version
>   (`manifest.lock`); no drift between agents or over time.
> - **Correct by construction** — the right Java 11+ runtime, the ScaResolver
>   `Configuration.yml`, and the KICS query assets are already in place.
>
> **One thing the image can't bundle: your project's build toolchain.** SCA
> Resolver resolves dependencies *locally*, so for the `sca` engine the scanned
> project's package managers (node/npm, python/pip, maven, gradle, nuget, go, …)
> must be present in the **execution environment**. These vary per project and are
> intentionally **not** in the image — add them in your CI before the scan step
> (e.g. `setup-node` + `setup-python`, or an `apt`/`apk` step) or build a
> downstream image `FROM` the fat image. The other four engines need nothing extra.

> ### ⚠️ Not recommended: thin image / standalone binary + manual tool install
> `…:latest` (slim) or the release binaries, with you installing `cx`, `ScaResolver`,
> `kics`, `2ms`, `CxConsolePlugin`, and **Java 11+** on each agent yourself.
>
> This is a **significantly less recommended** path. You take on installing and
> **version-managing five separate tools**, wiring up the ScaResolver
> `Configuration.yml` and the KICS query assets, and ensuring a Java 11+ runtime —
> on every agent. It means more moving parts, version drift, and more failure modes,
> and you lose the fat image's digest pinning. Use it **only** when policy forbids the
> fat image (e.g. a locked-down base image you must extend) — and even then, prefer
> `FROM ghcr.io/cx-michael-pogrebisky/cx-onprem-orchestrator:fat` as your base.

## Auto-detection

Detection precedence (avoids the shared `CI=true` collision):
`GITHUB_ACTIONS → GITLAB_CI → TF_BUILD (Azure) → TEAMCITY_VERSION → bamboo_* →
BITBUCKET_BUILD_NUMBER → JENKINS_URL → BUILDKITE → CIRCLECI → CODEBUILD_BUILD_ID →
TRAVIS → DRONE → SEMAPHORE → APPVEYOR → CF_BUILD_ID → else local`. Run
`cx-onprem-orchestrator detect` to see what it resolved.

Provide secrets as environment variables (never on the command line): `CX1_APIKEY`
(SCA/containers), `CXSAST_URL` / `CXSAST_USERNAME` / `CXSAST_PASSWORD` (CxSAST) — or
client-credentials, see [authentication.md](authentication.md). Reports survive a
threshold breach (the report-collection barrier), so upload them with an always-run step.

## Per-system snippets (all fat-image-first)

| CI system | Auto-detected | Snippet |
|---|---|---|
| GitHub Actions | ✅ | [ci/github.md](ci/github.md) |
| GitLab CI | ✅ | [ci/gitlab.md](ci/gitlab.md) |
| Azure DevOps Pipelines | ✅ | [ci/azure.md](ci/azure.md) |
| Jenkins (declarative) | ✅ | [ci/jenkins.md](ci/jenkins.md) |
| Bamboo | ✅ | [ci/bamboo.md](ci/bamboo.md) |
| Bitbucket Pipelines | ✅ | [ci/bitbucket.md](ci/bitbucket.md) |
| TeamCity | ✅ | [ci/teamcity.md](ci/teamcity.md) |
| Buildkite | ✅ | [ci/buildkite.md](ci/buildkite.md) |
| AWS CodeBuild | ✅ | [ci/codebuild.md](ci/codebuild.md) |
| CircleCI | ✅ | [ci/circleci.md](ci/circleci.md) |
| Travis CI | ✅ | [ci/travis.md](ci/travis.md) |
| Drone CI (and Harness CI) | ✅ | [ci/drone.md](ci/drone.md) |
| Semaphore | ✅ | [ci/semaphore.md](ci/semaphore.md) |
| AppVeyor | ✅ | [ci/appveyor.md](ci/appveyor.md) |
| Codefresh | ✅ | [ci/codefresh.md](ci/codefresh.md) |

> **Windows agents:** the snippets above use the `linux/amd64` fat image. On a
> **Windows** agent — and necessarily on **Windows Server 2016**, where no
> container runtime can run a Linux image — run the **native Windows binary**
> instead. See **[ci/windows.md](ci/windows.md)**.

## Any other CI (e.g. Google Cloud Build, Woodpecker)

If your system isn't auto-detected, run the **fat image** the same way and supply
context via the generic override env vars (or the `--branch` / `--source` flags):

```
CXSCAN_BRANCH=<branch>  CXSCAN_COMMIT=<sha>  CXSCAN_REPO=<url>  CXSCAN_WORKSPACE=<dir>
```

These are honored whenever no specific CI provider is detected.
