# Buildkite

`cx-onprem-orchestrator` auto-detects Buildkite (`BUILDKITE=true`), reading
branch/commit/repo from `BUILDKITE_BRANCH`/`BUILDKITE_COMMIT`/`BUILDKITE_REPO` and
the workspace from `BUILDKITE_BUILD_CHECKOUT_PATH`.

> **Docker or Podman** — the fat image runs identically under either; see
> [Container runtime](../ci.md#container-runtime--docker-or-podman). Buildkite has no
> hosted container directive to swap, so the `docker` plugin uses Docker. On
> **self-hosted agents you install Podman or Docker as you choose** and call it from a
> `command:` step (`podman run …` / `docker run …`).

> **Use the fat image** — `ghcr.io/cx-michael-pogrebisky/cx-onprem-orchestrator:fat`
> bundles every engine tool (digest-pinned). Installing the tools on the agent
> yourself is a [significantly less recommended path](../ci.md#image-choice).

```yaml
# .buildkite/pipeline.yml — runs inside the fat image via the docker plugin
steps:
  - label: ":checkmarx: scan"
    command: >
      cx-onprem-orchestrator run
      --scanners sast,sca,kics,secrets
      --threshold "sast-critical=1;sca-high=5;secrets-total=1"
      --output-path reports
    plugins:
      - docker#v5.11.0:
          image: ghcr.io/cx-michael-pogrebisky/cx-onprem-orchestrator:fat
          environment:
            - CX1_APIKEY
            - CXSAST_URL
            - CXSAST_USERNAME
            - CXSAST_PASSWORD
    artifact_paths: "reports/**"
# Provide CX1_APIKEY / CXSAST_* as Buildkite secrets (e.g. via the agent's secret store).
```

## Windows agents

The fat image is `linux/amd64` and can't run on a Windows host. **Buildkite Hosted
agents are Linux + macOS only** — there's no hosted Windows — so the snippet above is
the cloud path. For Windows you run the **native binary** on a **self-hosted Buildkite
agent on Windows**; see [windows.md](./windows.md) for the full setup.

Tag the self-hosted agent (e.g. `os=windows`) and target it with `agents:`, then run
`cx-onprem-orchestrator.exe` via `cmd`/`powershell` (secrets via env, never on argv):

```yaml
# self-hosted Windows agent — native binary, no container
steps:
  - label: ":checkmarx: scan (windows)"
    agents:
      os: windows
    command: >
      C:\cx\cx-onprem-orchestrator.exe run
      --scanners sast,sca,kics,secrets
      --threshold "sast-critical=1;sca-high=5;secrets-total=1"
      --sca-resolver C:\cx\sca\ScaResolver.exe
      --sast-path C:\cx\CxConsolePlugin
      --sast-java C:\Java\jdk-17\bin\java.exe
      --output-path reports
    artifact_paths: "reports/**"
# CX1_APIKEY / CXSAST_* come from the agent's secret store / env — never on the command line.
```
