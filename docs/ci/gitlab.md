# GitLab CI

`cx-onprem-orchestrator` auto-detects GitLab CI (branch/commit/repo from
`CI_COMMIT_*`/`CI_PROJECT_DIR`). Provide `CX1_APIKEY` and `CXSAST_*` as masked
CI/CD variables.

> **Docker or Podman?** The fat image runs identically under either — see
> [ci.md → Container runtime](../ci.md#container-runtime--docker-or-podman). The
> `image:` keyword on **shared** runners is Docker-backed and not swappable; to run
> under **Podman** use a **self-hosted** runner where you pick the executor/runtime
> (install Podman; shell executor, then `podman run …`).

```yaml
checkmarx:
  image: ghcr.io/cx-michael-pogrebisky/cx-onprem-orchestrator:fat
  variables: { GIT_DEPTH: "0" }
  script:
    - >
      cx-onprem-orchestrator run
      --scanners sast,sca,kics,secrets
      --threshold "sast-critical=1;sca-high=5;secrets-total=1"
      --sca-resolver /opt/sca/ScaResolver
      --sast-path /opt/cxconsole/runCxConsole.sh
      --output-path ./reports
  artifacts:
    when: always
    paths: [reports/]
```

## Windows agents

The fat image is `linux/amd64` and can't run on a Windows host, so on Windows run
the **native binary** — see **[windows.md](./windows.md)** for full setup. GitLab
offers hosted Windows (`saas-windows-medium-amd64`, Windows Server 2022; SaaS Windows
is **beta**) or a self-hosted Windows runner. Select it by tag and run via PowerShell;
inject secrets as masked CI/CD variables (never on argv):

```yaml
checkmarx-windows:
  tags: [saas-windows-medium-amd64]   # or a self-hosted Windows runner's tag
  script:
    - >
      C:\cx\cx-onprem-orchestrator.exe run
      --scanners sast,sca,kics,secrets
      --threshold "sast-critical=1;sca-high=5;secrets-total=1"
      --sast-java C:\Java\jdk-17\bin\java.exe
      --sca-resolver C:\cx\sca\ScaResolver.exe
      --sast-path C:\cx\CxConsolePlugin
      --output-path reports
  artifacts:
    when: always
    paths: [reports/]
```
