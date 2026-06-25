# Semaphore

`cx-onprem-orchestrator` auto-detects Semaphore (`SEMAPHORE=true`), reading
branch/commit/repo from `SEMAPHORE_GIT_BRANCH`/`SEMAPHORE_GIT_SHA`/
`SEMAPHORE_GIT_REPO_SLUG` and the workspace from `SEMAPHORE_GIT_DIR`.

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
