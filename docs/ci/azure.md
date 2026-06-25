# Azure DevOps Pipelines

`cx-onprem-orchestrator` auto-detects Azure Pipelines (`TF_BUILD`), reading
branch/commit/repo from `BUILD_SOURCEBRANCH`/`BUILD_SOURCEVERSION`/
`BUILD_REPOSITORY_URI` and the workspace from `BUILD_SOURCESDIRECTORY`. Provide
`CX1_APIKEY`/`CXSAST_*` as secret pipeline variables.

```yaml
- script: |
    docker run --rm -v "$(Build.SourcesDirectory)":/work -w /work \
      -e CX1_APIKEY -e CXSAST_URL -e CXSAST_USERNAME -e CXSAST_PASSWORD \
      ghcr.io/cx-michael-pogrebisky/cx-onprem-orchestrator:fat run \
        --scanners sast,sca,kics,secrets \
        --threshold "sast-critical=1;sca-high=5;secrets-total=1" \
        --sca-resolver /opt/sca/ScaResolver \
        --sast-path /opt/cxconsole/runCxConsole.sh \
        --output-path /work/reports
  env:
    CX1_APIKEY: $(CX1_APIKEY)
    CXSAST_URL: $(CXSAST_URL)
    CXSAST_USERNAME: $(CXSAST_USERNAME)
    CXSAST_PASSWORD: $(CXSAST_PASSWORD)
  displayName: Checkmarx scans
```
