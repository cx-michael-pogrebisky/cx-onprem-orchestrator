# Travis CI

`cx-onprem-orchestrator` auto-detects Travis CI (`TRAVIS=true`), reading
branch/commit/repo from `TRAVIS_BRANCH` (or `TRAVIS_PULL_REQUEST_BRANCH`)/
`TRAVIS_COMMIT`/`TRAVIS_REPO_SLUG` and the workspace from `TRAVIS_BUILD_DIR`.

> **Docker or Podman:** the fat image runs identically under either — see
> [Container runtime](../ci.md#container-runtime--docker-or-podman). On a Travis
> **Linux** worker, `services: [docker]` gives you Docker; to use Podman instead,
> install it via the worker's package manager and call `podman run`, or just use the
> bundled Docker. *(The container path is Linux-only; on a Windows worker use the
> native binary — see below.)*

> **Use the fat image** (all engine tools bundled, digest-pinned). Installing the
> tools on the worker yourself is a
> [significantly less recommended path](../ci.md#image-choice).

```yaml
# .travis.yml
services: [docker]
script:
  - >
    docker run --rm -v "$TRAVIS_BUILD_DIR":/work -w /work
    -e CX1_APIKEY -e CXSAST_URL -e CXSAST_USERNAME -e CXSAST_PASSWORD
    ghcr.io/cx-michael-pogrebisky/cx-onprem-orchestrator:fat run
    --scanners sast,sca,kics,secrets
    --threshold "sast-critical=1;sca-high=5;secrets-total=1"
    --output-path /work/reports
# Define CX1_APIKEY / CXSAST_* as encrypted Travis environment variables.
```

## Windows agents

The fat image is `linux/amd64` and can't run on a Windows host, so on a Travis
**Windows** worker (`os: windows`, Windows Server; early-release/limited) use the
**native Windows binary** — see [ci/windows.md](./windows.md) for full setup. Travis's
Windows shell is **Git Bash**; call `powershell` for PowerShell. Inject secrets via
encrypted env vars (never on argv):

```yaml
# .travis.yml
os: windows
script:
  - >
    powershell -Command "C:\cx\cx-onprem-orchestrator.exe run
    --scanners sast,sca,kics,secrets
    --threshold 'sast-critical=1;sca-high=5;secrets-total=1'
    --sast-java C:\Java\jdk-17\bin\java.exe
    --sca-resolver C:\cx\sca\ScaResolver.exe
    --sast-path C:\cx\CxConsolePlugin
    --output-path reports"
# CX1_APIKEY / CXSAST_* come from encrypted Travis env vars, not the command line.
```
