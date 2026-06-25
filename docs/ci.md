# CI integration

`cx-onprem-orchestrator` auto-detects the CI system and extracts branch / commit /
repo / workspace, so you usually only pass `--scanners` and `--threshold`. The
single static binary (or the fat image) runs on every system below.

Detection precedence (avoids the shared `CI=true` collision):
`GITHUB_ACTIONS → GITLAB_CI → TF_BUILD (Azure) → TEAMCITY_VERSION → bamboo_* →
BITBUCKET_BUILD_NUMBER → JENKINS_URL → local`. Run `cx-onprem-orchestrator detect`
to see what it resolved.

Provide secrets as environment variables (never on the command line):
`CX1_APIKEY` (SCA/containers), `CXSAST_URL` / `CXSAST_USERNAME` / `CXSAST_PASSWORD`
(CxSAST). Reports survive a threshold breach (the report-collection barrier), so
upload them with an always-run step.

---

## GitHub Actions

```yaml
jobs:
  checkmarx:
    runs-on: ubuntu-latest
    container: ghcr.io/cx-michael-pogrebisky/cx-onprem-orchestrator:fat   # tools bundled
    steps:
      - uses: actions/checkout@v4
      - env:
          CX1_APIKEY:      ${{ secrets.CX1_APIKEY }}
          CXSAST_URL:      ${{ vars.CXSAST_URL }}
          CXSAST_USERNAME: ${{ secrets.CXSAST_USERNAME }}
          CXSAST_PASSWORD: ${{ secrets.CXSAST_PASSWORD }}
        run: |
          cx-onprem-orchestrator run \
            --scanners sast,sca,kics,secrets \
            --threshold "sast-critical=1;sca-high=5;iac-security-low=10;secrets-total=1" \
            --sca-resolver /opt/sca/ScaResolver \
            --sast-path /opt/cxconsole/runCxConsole.sh \
            --output-path ./reports
      - if: always()
        uses: actions/upload-artifact@v4
        with: { name: cxoo-reports, path: ./reports }
```

## GitLab CI

```yaml
checkmarx:
  image: ghcr.io/cx-michael-pogrebisky/cx-onprem-orchestrator:fat
  variables: { GIT_DEPTH: "0" }
  script:
    - cx-onprem-orchestrator run --scanners sast,sca,kics,secrets
        --threshold "sast-critical=1;sca-high=5;secrets-total=1"
        --sca-resolver /opt/sca/ScaResolver
        --sast-path /opt/cxconsole/runCxConsole.sh
        --output-path ./reports
  artifacts:
    when: always
    paths: [reports/]
  # CX1_APIKEY / CXSAST_* provided as masked CI/CD variables.
```

## Azure DevOps Pipelines

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
    CXSAST_PASSWORD: $(CXSAST_PASSWORD)
  displayName: Checkmarx scans
```

## Jenkins (declarative)

```groovy
pipeline {
  agent { docker { image 'ghcr.io/cx-michael-pogrebisky/cx-onprem-orchestrator:fat'; args '-v $WORKSPACE:/work -w /work' } }
  environment { CX1_APIKEY = credentials('cx1-apikey'); CXSAST_PASSWORD = credentials('cxsast-password') }
  stages {
    stage('Checkmarx') {
      steps {
        sh '''cx-onprem-orchestrator run \
          --scanners sast,sca,kics,secrets \
          --threshold "sast-critical=1;sca-high=5;secrets-total=1" \
          --sca-resolver /opt/sca/ScaResolver \
          --sast-path /opt/cxconsole/runCxConsole.sh \
          --output-path reports'''
      }
    }
  }
  post { always { archiveArtifacts artifacts: 'reports/**', allowEmptyArchive: true } }
}
```

## Bamboo

Add a **Docker** task running `ghcr.io/cx-michael-pogrebisky/cx-onprem-orchestrator:fat` with the
container command `run --scanners ... --threshold "..."` and the workspace mounted
at `/work`. Expose `CX1_APIKEY`/`CXSAST_*` as plan variables. Branch/commit/repo are
read from `bamboo_planRepository_*`.

## Bitbucket Pipelines

```yaml
pipelines:
  default:
    - step:
        name: Checkmarx
        image: ghcr.io/cx-michael-pogrebisky/cx-onprem-orchestrator:fat
        script:
          - cx-onprem-orchestrator run --scanners sast,sca,kics,secrets
              --threshold "sast-critical=1;sca-high=5;secrets-total=1"
              --sca-resolver /opt/sca/ScaResolver
              --sast-path /opt/cxconsole/runCxConsole.sh
              --output-path reports
        artifacts: [reports/**]
        # CX1_APIKEY / CXSAST_* set as repository/workspace variables.
```

## TeamCity

TeamCity exposes only `BUILD_VCS_NUMBER` as an env var — branch/repo/checkout are
*configuration parameters* that are not auto-exported. The orchestrator falls back
to `git` introspection and logs that it did so. To set them explicitly, add build
parameters:

```
env.CXSCAN_BRANCH = %teamcity.build.branch%
env.CXSCAN_REPO   = %vcsroot.url%
```

Then run a **Docker** build step with image `ghcr.io/cx-michael-pogrebisky/cx-onprem-orchestrator:fat`
and command `run --scanners ... --threshold "..." --output-path reports`, exposing
`CX1_APIKEY`/`CXSAST_*` as (password-typed) environment parameters.
