# Bitbucket Pipelines

`cx-onprem-orchestrator` auto-detects Bitbucket Pipelines (`BITBUCKET_BUILD_NUMBER`),
reading branch/commit/repo from `BITBUCKET_BRANCH`/`BITBUCKET_COMMIT`/
`BITBUCKET_GIT_HTTP_ORIGIN` and the workspace from `BITBUCKET_CLONE_DIR`. Set
`CX1_APIKEY` / `CXSAST_*` as repository or workspace variables (secured).

> **Docker or Podman:** the fat image runs identically under either — see
> [ci.md → Container runtime](../ci.md#container-runtime--docker-or-podman). But
> **Cloud Pipelines run on Docker (Linux) and cannot use Podman**; for Podman (or
> Windows) use a **self-hosted runner**.

```yaml
pipelines:
  default:
    - step:
        name: Checkmarx
        image: ghcr.io/cx-michael-pogrebisky/cx-onprem-orchestrator:fat
        script:
          - >
            cx-onprem-orchestrator run
            --scanners sast,sca,kics,secrets
            --threshold "sast-critical=1;sca-high=5;secrets-total=1"
            --sca-resolver /opt/sca/ScaResolver
            --sast-path /opt/cxconsole/runCxConsole.sh
            --output-path reports
        artifacts:
          - reports/**
```

## Windows agents

Bitbucket **Cloud** Pipelines are **Linux/Docker only** — there is no hosted
Windows. The `linux/amd64` fat image above is the cloud path. For Windows, run a
**self-hosted runner** on the Windows host with the **native Windows binary** —
see [ci/windows.md](./windows.md) for the full setup. On that self-hosted runner
(PowerShell), inject secrets via env/credentials (never on argv) and run:

```powershell
C:\cx\cx-onprem-orchestrator.exe run `
  --scanners sast,sca,kics,secrets `
  --threshold "sast-critical=1;sca-high=5;secrets-total=1" `
  --sast-java C:\Java\jdk-17\bin\java.exe `
  --sca-resolver C:\cx\sca\ScaResolver.exe `
  --sast-path C:\cx\CxConsolePlugin `
  --output-path reports
```
