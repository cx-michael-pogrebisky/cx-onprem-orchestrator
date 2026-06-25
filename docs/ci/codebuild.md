# AWS CodeBuild

`cx-onprem-orchestrator` auto-detects AWS CodeBuild (`CODEBUILD_BUILD_ID`), reading
the branch from `CODEBUILD_WEBHOOK_HEAD_REF` (falling back to
`CODEBUILD_SOURCE_VERSION`), the commit from `CODEBUILD_RESOLVED_SOURCE_VERSION`,
the repo from `CODEBUILD_SOURCE_REPO_URL`, and the workspace from `CODEBUILD_SRC_DIR`.

> **Use the fat image** — set it as the CodeBuild project's container image so all
> engine tools are present (digest-pinned). Installing the tools in `install:`
> yourself is a [significantly less recommended path](../ci.md#image-choice).

```yaml
# buildspec.yml — CodeBuild project Environment image:
#   ghcr.io/cx-michael-pogrebisky/cx-onprem-orchestrator:fat
version: 0.2
phases:
  build:
    commands:
      - >
        cx-onprem-orchestrator run
        --scanners sast,sca,kics,secrets
        --threshold "sast-critical=1;sca-high=5;secrets-total=1"
        --output-path reports
artifacts:
  files:
    - reports/**/*
# Provide CX1_APIKEY / CXSAST_* via Secrets Manager (env-vars-from-secrets-manager).
```

If you cannot set a custom project image, run the fat image with Docker instead and
mount `"$CODEBUILD_SRC_DIR":/work`.
