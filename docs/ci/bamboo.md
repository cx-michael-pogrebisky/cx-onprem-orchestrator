# Bamboo

`cx-onprem-orchestrator` auto-detects Bamboo (`bamboo_*`), reading branch/commit/
repo from `bamboo_planRepository_branchName`/`_revision`/`_repositoryUrl` and the
workspace from `bamboo_build_working_directory`.

> **Docker or Podman:** the fat image is a standard OCI image and runs identically
> under either — see [ci.md → Container runtime](../ci.md#container-runtime--docker-or-podman).
> Bamboo agents are **your own remote agents**, so the runtime is fully selectable:
> install **Podman or Docker** as you prefer. The **Docker** task assumes a Docker
> CLI; for Podman, alias `docker=podman` or run `podman run …` in a **Script** task.

Add a **Docker** task that runs the fat image with the workspace mounted at
`/work`, and expose `CX1_APIKEY` / `CXSAST_*` as (masked) plan variables.

- **Docker image:** `ghcr.io/cx-michael-pogrebisky/cx-onprem-orchestrator:fat`
- **Container command:**
  ```
  run --scanners sast,sca,kics,secrets \
      --threshold "sast-critical=1;sca-high=5;secrets-total=1" \
      --sca-resolver /opt/sca/ScaResolver \
      --sast-path /opt/cxconsole/runCxConsole.sh \
      --output-path /work/reports
  ```
- **Volume:** map the build working directory to `/work`.
- **Environment variables:** `CX1_APIKEY`, `CXSAST_URL`, `CXSAST_USERNAME`, `CXSAST_PASSWORD`.

Artifact-define `reports/**` so results are retained even when the gate fails.

## Windows agents

The fat image is `linux/amd64` and can't run on a Windows host. Bamboo has **no
vendor-hosted runners** — Windows means a **self-hosted remote agent on a Windows
host**, so run the **native Windows binary** there. Keep the Linux fat-image Docker
task above for your Linux agents. Full setup: **[./windows.md](./windows.md)**.

Add a **Script** task (interpreter **cmd** or **PowerShell**) on a Windows-bound
agent. Inject secrets as **masked plan variables** (env), never on argv:

```bat
C:\cx\cx-onprem-orchestrator.exe run ^
  --scanners sast,sca,kics,secrets ^
  --threshold "sast-critical=1;sca-high=5;secrets-total=1" ^
  --sast-java C:\Java\jdk-17\bin\java.exe ^
  --sca-resolver C:\cx\sca\ScaResolver.exe ^
  --sast-path C:\cx\CxConsolePlugin ^
  --output-path reports
```

Bind the task to the Windows agent via an agent **capability/label**; `cmd` has no
line-continuation across tasks, so keep the run on one logical command (`^` above).
