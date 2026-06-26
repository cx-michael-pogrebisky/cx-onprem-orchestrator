# Codefresh

`cx-onprem-orchestrator` auto-detects Codefresh (`CF_BUILD_ID`), reading
branch/commit/repo from `CF_BRANCH`/`CF_REVISION`/`CF_COMMIT_URL` and the workspace
from `CF_VOLUME_PATH`.

> **Docker or Podman:** the fat image is a standard OCI image and runs identically
> under either — see [Container runtime](../ci.md#container-runtime--docker-or-podman).
> The standard Codefresh runtime is **Linux/Docker** and isn't swappable per step; for
> **Podman**, use a **self-hosted runtime** where you control the engine.

> **Use the fat image** as the step image — all engine tools are bundled
> (digest-pinned). Installing them yourself is a
> [significantly less recommended path](../ci.md#image-choice).

```yaml
# codefresh.yml
version: "1.0"
steps:
  scan:
    type: freestyle
    image: ghcr.io/cx-michael-pogrebisky/cx-onprem-orchestrator:fat
    working_directory: ${{main_clone}}
    environment:
      - CX1_APIKEY=${{CX1_APIKEY}}
      - CXSAST_URL=${{CXSAST_URL}}
      - CXSAST_USERNAME=${{CXSAST_USERNAME}}
      - CXSAST_PASSWORD=${{CXSAST_PASSWORD}}
    commands:
      - >
        cx-onprem-orchestrator run
        --scanners sast,sca,kics,secrets
        --threshold "sast-critical=1;sca-high=5;secrets-total=1"
        --output-path reports
# Provide CX1_APIKEY / CXSAST_* as Codefresh shared-configuration / encrypted vars.
```

## Windows agents

The fat image is `linux/amd64`, so it can't run on a Windows host. The Codefresh
**cloud is Linux-only** — a hosted Windows Server VM runtime exists but is gated
(Incubation; enabled via Codefresh sales). The snippet above remains the cloud path.

For Windows, run the **native Windows binary** on a **self-hosted Windows
runner/agent** — full setup in **[windows.md](./windows.md)**. Inject secrets via
env/credentials (never on argv) and point the orchestrator at the native tools:

```powershell
$env:CX1_APIKEY = '...'; $env:CXSAST_URL = '...'
$env:CXSAST_USERNAME = '...'; $env:CXSAST_PASSWORD = '...'
C:\cx\cx-onprem-orchestrator.exe run `
  --scanners sast,sca,kics,secrets `
  --threshold "sast-critical=1;sca-high=5;secrets-total=1" `
  --sast-java C:\Java\jdk-17\bin\java.exe `
  --sca-resolver C:\cx\sca\ScaResolver.exe `
  --sast-path C:\cx\CxConsolePlugin `
  --output-path reports
```
