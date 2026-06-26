# Azure DevOps Pipelines

`cx-onprem-orchestrator` auto-detects Azure Pipelines (`TF_BUILD`), reading
branch/commit/repo from `BUILD_SOURCEBRANCH`/`BUILD_SOURCEVERSION`/
`BUILD_REPOSITORY_URI` and the workspace from `BUILD_SOURCESDIRECTORY`. Provide
`CX1_APIKEY`/`CXSAST_*` as secret pipeline variables.

> **Docker or Podman:** the fat image runs identically under either — see
> [ci.md → Container runtime](../ci.md#container-runtime--docker-or-podman).
> Microsoft-hosted container jobs are **Docker-backed** and can't be swapped to
> Podman; to use Podman, `podman run …` in a plain `script:` step (install it
> first) or use a **self-hosted** agent where you control the runtime.

```yaml
- script: |
    docker run --rm -v "$(Build.SourcesDirectory)":/work -w /work \
      -e CX1_APIKEY -e CXSAST_URL -e CXSAST_USERNAME -e CXSAST_PASSWORD \
      ghcr.io/cx-michael-pogrebisky/cx-onprem-orchestrator:fat run \
        --scanners sast,sca,kics,secrets \
        --threshold "sast-critical=1;sca-high=5;secrets-total=1" \
        --sca-resolver /opt/sca/ScaResolver \
        --sast-path /opt/cxconsole/runCxConsole.sh \
        --output-path /work/reports
  env:
    CX1_APIKEY: $(CX1_APIKEY)
    CXSAST_URL: $(CXSAST_URL)
    CXSAST_USERNAME: $(CXSAST_USERNAME)
    CXSAST_PASSWORD: $(CXSAST_PASSWORD)
  displayName: Checkmarx scans
```

## Windows agents

The `linux/amd64` fat image can't run on a Windows host, so on a Windows agent use
the **native binary** — full setup in [windows.md](./windows.md). Azure offers
hosted Windows agents (`windows-latest` = windows-2025, plus windows-2022 /
windows-2019); select one and run `cx-onprem-orchestrator.exe`:

```yaml
- job: checkmarx_windows
  pool:
    vmImage: 'windows-latest'   # or windows-2022 / windows-2019
  steps:
  - pwsh: |
      C:\cx\cx-onprem-orchestrator.exe run `
        --scanners sast,sca,kics,secrets `
        --threshold "sast-critical=1;sca-high=5;secrets-total=1" `
        --sast-java C:\Java\jdk-17\bin\java.exe `
        --sca-resolver C:\cx\sca\ScaResolver.exe `
        --sast-path C:\cx\CxConsolePlugin `
        --output-path reports
    env:
      CX1_APIKEY: $(CX1_APIKEY)
      CXSAST_URL: $(CXSAST_URL)
      CXSAST_USERNAME: $(CXSAST_USERNAME)
      CXSAST_PASSWORD: $(CXSAST_PASSWORD)
    displayName: Checkmarx scans (Windows)
```

Secrets stay in `env:` (never on argv). Use `script:` (cmd) instead of `pwsh:` if
you prefer cmd — note cmd has no line-continuation.
