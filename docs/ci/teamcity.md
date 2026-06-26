# TeamCity

TeamCity exposes only `BUILD_VCS_NUMBER` (commit) as an environment variable —
branch, repo URL, and checkout dir are *configuration parameters* that are NOT
auto-exported. `cx-onprem-orchestrator` falls back to `git` introspection and logs
that it did so. To pass them explicitly, add build parameters:

```
env.CXSCAN_BRANCH = %teamcity.build.branch%
env.CXSCAN_REPO   = %vcsroot.url%
```

> **Docker or Podman?** The fat image is a standard OCI image and runs identically
> under [Docker or Podman](../ci.md#container-runtime--docker-or-podman). The Docker
> build step runner below is bound to Docker and can't be pointed at Podman; on
> **self-hosted** agents you pick the runtime — install Podman or Docker and call
> `podman run …` / `docker run …` from a Command Line step instead.

Then add a **Docker** build step:

- **Image:** `ghcr.io/cx-michael-pogrebisky/cx-onprem-orchestrator:fat`
- **Command:**
  ```
  run --scanners sast,sca,kics,secrets \
      --threshold "sast-critical=1;sca-high=5;secrets-total=1" \
      --sca-resolver /opt/sca/ScaResolver \
      --sast-path /opt/cxconsole/runCxConsole.sh \
      --output-path reports
  ```
- Expose `CX1_APIKEY` / `CXSAST_URL` / `CXSAST_USERNAME` / `CXSAST_PASSWORD` as
  (password-typed) environment parameters.

## Windows agents

The fat image is `linux/amd64` and cannot run on a Windows host, so on Windows run the
**native binary** instead — see [windows.md](./windows.md) for the full setup. TeamCity
offers Windows agents both ways: JetBrains-hosted Windows cloud agents (TeamCity Cloud)
or self-hosted Windows agents. Add an agent requirement on `teamcity.agent.jvm.os.name`
(or an OS-tagged pool), keep secrets as password-typed env parameters (never on argv),
and use a **PowerShell** (or Command Line) build step:

```powershell
C:\cx\cx-onprem-orchestrator.exe run `
  --scanners sast,sca,kics,secrets `
  --threshold "sast-critical=1;sca-high=5;secrets-total=1" `
  --sast-java C:\Java\jdk-17\bin\java.exe `
  --sca-resolver C:\cx\sca\ScaResolver.exe `
  --sast-path C:\cx\CxConsolePlugin `
  --output-path reports
```
