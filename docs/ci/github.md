# GitHub Actions

`cx-onprem-orchestrator` auto-detects GitHub Actions and extracts branch/commit/
repo/workspace. Provide secrets as env vars (never on the command line); reports
survive a threshold breach, so upload them in an `if: always()` step.

> **Docker or Podman?** The fat image runs identically under either — see
> [ci.md → Container runtime](../ci.md#container-runtime--docker-or-podman).
> GitHub-hosted Ubuntu runners have **Podman preinstalled**, so `podman run …`
> works in a `run:` step; the job-level `container:` runtime is Docker/Moby and
> **can't be swapped to Podman** on hosted runners.

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

## Windows agents

The fat image is `linux/amd64` and **can't run on a Windows host** — on Windows,
run the **native binary** instead. See [windows.md](./windows.md) for the full
setup. GitHub offers **hosted Windows runners** (`windows-2019/2022/2025`,
`windows-latest` = 2025), so select one and run `cx-onprem-orchestrator.exe`:

```yaml
jobs:
  checkmarx-windows:
    runs-on: windows-2022          # or windows-2025 / windows-2019 / windows-latest
    steps:
      - uses: actions/checkout@v4
      - shell: pwsh
        env:
          CX1_APIKEY:      ${{ secrets.CX1_APIKEY }}
          CXSAST_URL:      ${{ vars.CXSAST_URL }}
          CXSAST_USERNAME: ${{ secrets.CXSAST_USERNAME }}
          CXSAST_PASSWORD: ${{ secrets.CXSAST_PASSWORD }}
        run: |
          C:\cx\cx-onprem-orchestrator.exe run `
            --scanners sast,sca,kics,secrets `
            --threshold "sast-critical=1;sca-high=5;secrets-total=1" `
            --sast-java C:\Java\jdk-17\bin\java.exe `
            --sca-resolver C:\cx\sca\ScaResolver.exe `
            --sast-path C:\cx\CxConsolePlugin `
            --output-path reports
```
