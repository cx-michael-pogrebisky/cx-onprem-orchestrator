# Drone CI

`cx-onprem-orchestrator` auto-detects Drone (`DRONE=true`), reading branch/commit/
repo from `DRONE_COMMIT_BRANCH`/`DRONE_COMMIT_SHA`/`DRONE_GIT_HTTP_URL` and the
workspace from `DRONE_WORKSPACE`. (Harness CI, which runs on the Drone runner, is
detected the same way.)

> **Docker or Podman:** the fat image is a standard OCI image and runs identically
> under either — see [ci.md → Container runtime](../ci.md#container-runtime--docker-or-podman).
> The `type: docker` pipeline's runtime is fixed by the runner, so you can't swap
> Podman into it; for Podman, use a **self-hosted** runner (Exec runner + Podman/Docker).

> **Use the fat image** as the step image — all engine tools are bundled
> (digest-pinned). Installing them yourself is a
> [significantly less recommended path](../ci.md#image-choice).

```yaml
# .drone.yml
kind: pipeline
type: docker
name: checkmarx
steps:
  - name: scan
    image: ghcr.io/cx-michael-pogrebisky/cx-onprem-orchestrator:fat
    environment:
      CX1_APIKEY:      { from_secret: cx1_apikey }
      CXSAST_URL:      { from_secret: cxsast_url }
      CXSAST_USERNAME: { from_secret: cxsast_username }
      CXSAST_PASSWORD: { from_secret: cxsast_password }
    commands:
      - >
        cx-onprem-orchestrator run
        --scanners sast,sca,kics,secrets
        --threshold "sast-critical=1;sca-high=5;secrets-total=1"
        --output-path reports
```

## Windows agents

The fat image is `linux/amd64`, so it can't run on a Windows host — on Windows use
the **native binary** (full setup in [windows.md](./windows.md)). Drone Cloud is
Linux-only; the Linux snippet above stays your cloud path. For Windows, register a
**self-hosted Windows runner** and run `cx-onprem-orchestrator.exe` natively. Inject
secrets via `environment:`/`from_secret` (never on argv):

```yaml
# .drone.yml (self-hosted Windows runner)
kind: pipeline
type: exec
name: checkmarx-windows
platform: { os: windows, arch: amd64 }
steps:
  - name: scan
    environment:
      CX1_APIKEY:      { from_secret: cx1_apikey }
      CXSAST_URL:      { from_secret: cxsast_url }
      CXSAST_USERNAME: { from_secret: cxsast_username }
      CXSAST_PASSWORD: { from_secret: cxsast_password }
    commands:
      - >
        C:\cx\cx-onprem-orchestrator.exe run
        --scanners sast,sca,kics,secrets
        --threshold "sast-critical=1;sca-high=5;secrets-total=1"
        --sast-java C:\Java\jdk-17\bin\java.exe
        --sca-resolver C:\cx\sca\ScaResolver.exe
        --sast-path C:\cx\CxConsolePlugin
        --output-path reports
```
