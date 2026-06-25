# Bitbucket Pipelines

`cx-onprem-orchestrator` auto-detects Bitbucket Pipelines (`BITBUCKET_BUILD_NUMBER`),
reading branch/commit/repo from `BITBUCKET_BRANCH`/`BITBUCKET_COMMIT`/
`BITBUCKET_GIT_HTTP_ORIGIN` and the workspace from `BITBUCKET_CLONE_DIR`. Set
`CX1_APIKEY` / `CXSAST_*` as repository or workspace variables (secured).

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
