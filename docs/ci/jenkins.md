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

## Windows build agents

The `docker` agent above assumes a **Linux** agent (the fat image is a
`linux/amd64` image). On a **Windows** agent — including **Windows Server 2016**,
where neither Podman nor Docker can run our Linux image (no WSL2; see
[windows.md](windows.md)) — use the **native Windows binary** instead of a
container, and `bat` steps instead of `sh`:

```groovy
pipeline {
  agent { label 'windows' }
  environment {
    CX1_APIKEY      = credentials('cx1-apikey')        // masked; injected by name
    CXSAST_PASSWORD = credentials('cxsast-password')
    CXSAST_URL      = 'http://cxsast.internal'
    CXSAST_USERNAME = 'svc-checkmarx'
  }
  stages {
    stage('Checkmarx') {
      steps {
        // ONE line: cmd.exe has no backslash line-continuation. Secrets stay in
        // env (masked by Jenkins), never on argv.
        bat 'C:\\cx\\cx-onprem-orchestrator.exe run --scanners sast,sca,kics,secrets,containers --sast-team "CxServer/SP" --threshold "sast-critical=1;sca-high=5;secrets-total=1" --sca-resolver C:\\cx\\sca\\ScaResolver.exe --sast-path C:\\cx\\CxConsolePlugin --sast-java C:\\Java\\jdk-17\\bin\\java.exe --output-path reports'
      }
    }
  }
  post { always { archiveArtifacts artifacts: 'reports/**', allowEmptyArchive: true } }
}
```

See **[windows.md](windows.md)** for the full Windows-agent setup (what to
install, why containers don't work on Server 2016, and the gotchas).
