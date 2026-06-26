# AWS CodeBuild

`cx-onprem-orchestrator` auto-detects AWS CodeBuild (`CODEBUILD_BUILD_ID`), reading
the branch from `CODEBUILD_WEBHOOK_HEAD_REF` (falling back to
`CODEBUILD_SOURCE_VERSION`), the commit from `CODEBUILD_RESOLVED_SOURCE_VERSION`,
the repo from `CODEBUILD_SOURCE_REPO_URL`, and the workspace from `CODEBUILD_SRC_DIR`.

> **Docker or Podman?** The `linux/amd64` fat image runs identically under either —
> see [Container runtime](../ci.md#container-runtime--docker-or-podman). On **Linux**
> CodeBuild, Podman runs daemonless without `privilegedMode`. But CodeBuild's
> **Windows** environments are **Windows containers** and cannot run our linux/amd64
> image — use the **native binary** there (see [Windows agents](#windows-agents)).

> **Use the fat image** — set it as the CodeBuild project's container image so all
> engine tools are present (digest-pinned). Installing the tools in `install:`
> yourself is a [significantly less recommended path](../ci.md#image-choice).

```yaml
# buildspec.yml — CodeBuild project Environment image:
#   ghcr.io/cx-michael-pogrebisky/cx-onprem-orchestrator:fat
version: 0.2
phases:
  build:
    commands:
      - >
        cx-onprem-orchestrator run
        --scanners sast,sca,kics,secrets
        --threshold "sast-critical=1;sca-high=5;secrets-total=1"
        --output-path reports
artifacts:
  files:
    - reports/**/*
# Provide CX1_APIKEY / CXSAST_* via Secrets Manager (env-vars-from-secrets-manager).
```

If you cannot set a custom project image, run the fat image with Docker instead and
mount `"$CODEBUILD_SRC_DIR":/work`.

## Windows agents

CodeBuild's Windows environments (`WINDOWS_SERVER_2022_CONTAINER`) are **Windows
containers** — they **cannot** run the `linux/amd64` fat image. Use the **native
Windows binary**; see **[ci/windows.md](./windows.md)** for the full setup.

```yaml
# buildspec.yml — project Environment: type WINDOWS_SERVER_2022_CONTAINER,
#   image aws/codebuild/windows-base:2022-1.0, on a reserved-capacity fleet.
#   Provide CX1_APIKEY / CXSAST_* via Secrets Manager (env-vars-from-secrets-manager).
version: 0.2
phases:
  build:
    commands:
      - |
        & "C:\cx\cx-onprem-orchestrator.exe" run `       # phase shell: powershell
          --scanners sast,sca,kics,secrets `
          --threshold "sast-critical=1;sca-high=5;secrets-total=1" `
          --sast-java "C:\Java\jdk-17\bin\java.exe" `
          --sca-resolver "C:\cx\sca\ScaResolver.exe" `
          --sast-path "C:\cx\CxConsolePlugin" `
          --output-path reports
artifacts:
  files:
    - reports/**/*
```

Set `shell: powershell` in the phase (CodeBuild Windows runs PowerShell). Inject
secrets via env only — never on argv.
