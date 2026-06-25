# CircleCI

`cx-onprem-orchestrator` auto-detects CircleCI (`CIRCLECI=true`), reading
branch/commit/repo from `CIRCLE_BRANCH`/`CIRCLE_SHA1`/`CIRCLE_REPOSITORY_URL` and the
workspace from `CIRCLE_WORKING_DIRECTORY`.

> **Use the fat image** as the job's executor image — all engine tools are bundled
> (digest-pinned). Installing them yourself is a
> [significantly less recommended path](../ci.md#image-choice).

```yaml
# .circleci/config.yml
version: 2.1
jobs:
  checkmarx:
    docker:
      - image: ghcr.io/cx-michael-pogrebisky/cx-onprem-orchestrator:fat
    steps:
      - checkout
      - run:
          name: Checkmarx scans
          command: >
            cx-onprem-orchestrator run
            --scanners sast,sca,kics,secrets
            --threshold "sast-critical=1;sca-high=5;secrets-total=1"
            --output-path reports
      - store_artifacts:
          path: reports
workflows:
  main:
    jobs: [checkmarx]
# Provide CX1_APIKEY / CXSAST_* via a CircleCI context or project env vars.
```
