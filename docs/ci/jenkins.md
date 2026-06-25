# Jenkins (declarative pipeline)

`cx-onprem-orchestrator` auto-detects Jenkins (`JENKINS_URL`), reading
branch/commit/repo from the Git plugin's `GIT_BRANCH`/`GIT_COMMIT`/`GIT_URL` and
the workspace from `WORKSPACE`. Store `CX1_APIKEY`/`CXSAST_PASSWORD` as credentials.

```groovy
pipeline {
  agent {
    docker {
      image 'ghcr.io/cx-michael-pogrebisky/cx-onprem-orchestrator:fat'
      args  '-v $WORKSPACE:/work -w /work'
    }
  }
  environment {
    CX1_APIKEY      = credentials('cx1-apikey')
    CXSAST_PASSWORD = credentials('cxsast-password')
    CXSAST_URL      = 'http://cxsast.internal'
    CXSAST_USERNAME = 'svc-checkmarx'
  }
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
