# GitHub Actions

`cx-onprem-orchestrator` auto-detects GitHub Actions and extracts branch/commit/
repo/workspace. Provide secrets as env vars (never on the command line); reports
survive a threshold breach, so upload them in an `if: always()` step.

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
