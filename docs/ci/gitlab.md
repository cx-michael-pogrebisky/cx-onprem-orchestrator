# GitLab CI

`cx-onprem-orchestrator` auto-detects GitLab CI (branch/commit/repo from
`CI_COMMIT_*`/`CI_PROJECT_DIR`). Provide `CX1_APIKEY` and `CXSAST_*` as masked
CI/CD variables.

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
