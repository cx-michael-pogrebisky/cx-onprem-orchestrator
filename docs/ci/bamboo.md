# Bamboo

`cx-onprem-orchestrator` auto-detects Bamboo (`bamboo_*`), reading branch/commit/
repo from `bamboo_planRepository_branchName`/`_revision`/`_repositoryUrl` and the
workspace from `bamboo_build_working_directory`.

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
