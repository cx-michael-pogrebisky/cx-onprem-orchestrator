# Drone CI

`cx-onprem-orchestrator` auto-detects Drone (`DRONE=true`), reading branch/commit/
repo from `DRONE_COMMIT_BRANCH`/`DRONE_COMMIT_SHA`/`DRONE_GIT_HTTP_URL` and the
workspace from `DRONE_WORKSPACE`. (Harness CI, which runs on the Drone runner, is
detected the same way.)

> **Use the fat image** as the step image — all engine tools are bundled
> (digest-pinned). Installing them yourself is a
> [significantly less recommended path](../ci.md#image-choice).

```yaml
# .drone.yml
kind: pipeline
type: docker
name: checkmarx
steps:
  - name: scan
    image: ghcr.io/cx-michael-pogrebisky/cx-onprem-orchestrator:fat
    environment:
      CX1_APIKEY:      { from_secret: cx1_apikey }
      CXSAST_URL:      { from_secret: cxsast_url }
      CXSAST_USERNAME: { from_secret: cxsast_username }
      CXSAST_PASSWORD: { from_secret: cxsast_password }
    commands:
      - >
        cx-onprem-orchestrator run
        --scanners sast,sca,kics,secrets
        --threshold "sast-critical=1;sca-high=5;secrets-total=1"
        --output-path reports
```
