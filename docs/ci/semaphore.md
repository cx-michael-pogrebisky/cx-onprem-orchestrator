# Semaphore

`cx-onprem-orchestrator` auto-detects Semaphore (`SEMAPHORE=true`), reading
branch/commit/repo from `SEMAPHORE_GIT_BRANCH`/`SEMAPHORE_GIT_SHA`/
`SEMAPHORE_GIT_REPO_SLUG` and the workspace from `SEMAPHORE_GIT_DIR`.

> **Docker or Podman:** the fat image is a standard OCI image, so it runs
> identically under [Docker or Podman](../ci.md#container-runtime--docker-or-podman).
> Semaphore **cloud** agents are **Linux/macOS only** and ship Docker; for Windows
> **or** Podman, use a **self-hosted** agent where you control the runtime.

> **Use the fat image** (all engine tools bundled, digest-pinned). Installing them
> on the agent yourself is a [significantly less recommended path](../ci.md#image-choice).

```yaml
# .semaphore/semaphore.yml
version: v1.0
name: checkmarx
agent:
  machine: { type: e1-standard-2, os_image: ubuntu2204 }
blocks:
  - name: scan
    task:
      secrets:
        - name: checkmarx        # exposes CX1_APIKEY / CXSAST_* as env vars
      jobs:
        - name: run
          commands:
            - checkout
            - >
              docker run --rm -v "$SEMAPHORE_GIT_DIR":/work -w /work
              -e CX1_APIKEY -e CXSAST_URL -e CXSAST_USERNAME -e CXSAST_PASSWORD
              ghcr.io/cx-michael-pogrebisky/cx-onprem-orchestrator:fat run
              --scanners sast,sca,kics,secrets
              --threshold "sast-critical=1;sca-high=5;secrets-total=1"
              --output-path /work/reports
```

## Windows agents

Semaphore **cloud** offers **Linux and macOS** agents only — there is **no hosted
Windows**. The `linux/amd64` fat image is the cloud path (snippet above). For
Windows, run the **native binary** on a **self-hosted Windows agent** — see
[ci/windows.md](./windows.md) for the full setup. Self-hosted Semaphore runs each
command in a fresh PowerShell, so use one continued command:

```powershell
C:\cx\cx-onprem-orchestrator.exe run `
  --scanners sast,sca,kics,secrets `
  --threshold "sast-critical=1;sca-high=5;secrets-total=1" `
  --sast-java C:\Java\jdk-17\bin\java.exe `
  --sca-resolver C:\cx\sca\ScaResolver.exe `
  --sast-path C:\cx\CxConsolePlugin `
  --output-path reports
```

Inject `CX1_APIKEY` / `CXSAST_*` via the agent's environment or a Semaphore secret
(never on argv).
