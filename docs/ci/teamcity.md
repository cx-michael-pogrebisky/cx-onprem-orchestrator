# TeamCity

TeamCity exposes only `BUILD_VCS_NUMBER` (commit) as an environment variable —
branch, repo URL, and checkout dir are *configuration parameters* that are NOT
auto-exported. `cx-onprem-orchestrator` falls back to `git` introspection and logs
that it did so. To pass them explicitly, add build parameters:

```
env.CXSCAN_BRANCH = %teamcity.build.branch%
env.CXSCAN_REPO   = %vcsroot.url%
```

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
