# AppVeyor

`cx-onprem-orchestrator` auto-detects AppVeyor (`APPVEYOR=True`), reading
branch/commit/repo from `APPVEYOR_REPO_BRANCH`/`APPVEYOR_REPO_COMMIT`/
`APPVEYOR_REPO_NAME` and the workspace from `APPVEYOR_BUILD_FOLDER`.

> **Use the fat image** (all engine tools bundled, digest-pinned). Installing them
> yourself is a [significantly less recommended path](../ci.md#image-choice).

```yaml
# appveyor.yml  (Linux image, Docker available)
image: Ubuntu2204
services:
  - docker
build_script:
  - sh: >
      docker run --rm -v "$APPVEYOR_BUILD_FOLDER":/work -w /work
      -e CX1_APIKEY -e CXSAST_URL -e CXSAST_USERNAME -e CXSAST_PASSWORD
      ghcr.io/cx-michael-pogrebisky/cx-onprem-orchestrator:fat run
      --scanners sast,sca,kics,secrets
      --threshold "sast-critical=1;sca-high=5;secrets-total=1"
      --output-path /work/reports
# Define CX1_APIKEY / CXSAST_* as (encrypted) AppVeyor environment variables.
```
