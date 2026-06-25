# Buildkite

`cx-onprem-orchestrator` auto-detects Buildkite (`BUILDKITE=true`), reading
branch/commit/repo from `BUILDKITE_BRANCH`/`BUILDKITE_COMMIT`/`BUILDKITE_REPO` and
the workspace from `BUILDKITE_BUILD_CHECKOUT_PATH`.

> **Use the fat image** — `ghcr.io/cx-michael-pogrebisky/cx-onprem-orchestrator:fat`
> bundles every engine tool (digest-pinned). Installing the tools on the agent
> yourself is a [significantly less recommended path](../ci.md#image-choice).

```yaml
# .buildkite/pipeline.yml — runs inside the fat image via the docker plugin
steps:
  - label: ":checkmarx: scan"
    command: >
      cx-onprem-orchestrator run
      --scanners sast,sca,kics,secrets
      --threshold "sast-critical=1;sca-high=5;secrets-total=1"
      --output-path reports
    plugins:
      - docker#v5.11.0:
          image: ghcr.io/cx-michael-pogrebisky/cx-onprem-orchestrator:fat
          environment:
            - CX1_APIKEY
            - CXSAST_URL
            - CXSAST_USERNAME
            - CXSAST_PASSWORD
    artifact_paths: "reports/**"
# Provide CX1_APIKEY / CXSAST_* as Buildkite secrets (e.g. via the agent's secret store).
```
