# Codefresh

`cx-onprem-orchestrator` auto-detects Codefresh (`CF_BUILD_ID`), reading
branch/commit/repo from `CF_BRANCH`/`CF_REVISION`/`CF_COMMIT_URL` and the workspace
from `CF_VOLUME_PATH`.

> **Use the fat image** as the step image — all engine tools are bundled
> (digest-pinned). Installing them yourself is a
> [significantly less recommended path](../ci.md#image-choice).

```yaml
# codefresh.yml
version: "1.0"
steps:
  scan:
    type: freestyle
    image: ghcr.io/cx-michael-pogrebisky/cx-onprem-orchestrator:fat
    working_directory: ${{main_clone}}
    environment:
      - CX1_APIKEY=${{CX1_APIKEY}}
      - CXSAST_URL=${{CXSAST_URL}}
      - CXSAST_USERNAME=${{CXSAST_USERNAME}}
      - CXSAST_PASSWORD=${{CXSAST_PASSWORD}}
    commands:
      - >
        cx-onprem-orchestrator run
        --scanners sast,sca,kics,secrets
        --threshold "sast-critical=1;sca-high=5;secrets-total=1"
        --output-path reports
# Provide CX1_APIKEY / CXSAST_* as Codefresh shared-configuration / encrypted vars.
```
