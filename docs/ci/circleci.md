# CircleCI

`cx-onprem-orchestrator` auto-detects CircleCI (`CIRCLECI=true`), reading
branch/commit/repo from `CIRCLE_BRANCH`/`CIRCLE_SHA1`/`CIRCLE_REPOSITORY_URL` and the
workspace from `CIRCLE_WORKING_DIRECTORY`.

> **Use the fat image** as the job's executor image — all engine tools are bundled
> (digest-pinned). Installing them yourself is a
> [significantly less recommended path](../ci.md#image-choice).

> **Docker or Podman?** The fat image runs identically under either — see
> [Container runtime](../ci.md#container-runtime--docker-or-podman). The `docker`
> executor's runtime is fixed by CircleCI and can't be swapped to Podman; for Podman
> use a **self-hosted runner** (or `podman run …` in a `machine`/runner step where
> installed).

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

## Windows agents

The fat image is `linux/amd64` and can't run on a Windows host. CircleCI offers a
hosted **Windows VM** executor (the `circleci/windows` orb — `win/server-2022`, also
`2019`) and self-hosted Windows runners; on both, run the **native binary**. See
[ci/windows.md](./windows.md) for the full agent setup.

```yaml
version: 2.1
orbs:
  win: circleci/windows@5.0
jobs:
  checkmarx-win:
    executor: win/server-2022          # or win/server-2019
    steps:
      - checkout
      - run:
          name: Checkmarx scans
          shell: powershell.exe
          # CX1_APIKEY / CXSAST_* injected via context/project env vars (never on argv)
          command: |
            C:\cx\cx-onprem-orchestrator.exe run `
              --scanners sast,sca,kics,secrets `
              --threshold "sast-critical=1;sca-high=5;secrets-total=1" `
              --sast-java C:\Java\jdk-17\bin\java.exe `
              --sca-resolver C:\cx\sca\ScaResolver.exe `
              --sast-path C:\cx\CxConsolePlugin `
              --output-path reports
      - store_artifacts:
          path: reports
```
