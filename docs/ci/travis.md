# Travis CI

`cx-onprem-orchestrator` auto-detects Travis CI (`TRAVIS=true`), reading
branch/commit/repo from `TRAVIS_BRANCH` (or `TRAVIS_PULL_REQUEST_BRANCH`)/
`TRAVIS_COMMIT`/`TRAVIS_REPO_SLUG` and the workspace from `TRAVIS_BUILD_DIR`.

> **Use the fat image** (all engine tools bundled, digest-pinned). Installing the
> tools on the worker yourself is a
> [significantly less recommended path](../ci.md#image-choice).

```yaml
# .travis.yml
services: [docker]
script:
  - >
    docker run --rm -v "$TRAVIS_BUILD_DIR":/work -w /work
    -e CX1_APIKEY -e CXSAST_URL -e CXSAST_USERNAME -e CXSAST_PASSWORD
    ghcr.io/cx-michael-pogrebisky/cx-onprem-orchestrator:fat run
    --scanners sast,sca,kics,secrets
    --threshold "sast-critical=1;sca-high=5;secrets-total=1"
    --output-path /work/reports
# Define CX1_APIKEY / CXSAST_* as encrypted Travis environment variables.
```
