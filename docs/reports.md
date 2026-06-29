# Reports & artifacts

`cx-onprem-orchestrator` collects every engine's report into a single output tree
**before** any threshold gating (the report-collection barrier), so artifacts
always exist — even when the run fails on a breach. CI steps can upload them with
an always-run step.

## Reports live OUTSIDE the scanned tree

Reports must never sit inside `--source`, or the scanners would pick them up — most
dangerously **2ms**, which would re-detect the secrets it just wrote into its own
report. So:

- **Default `--output-path` is a directory OUTSIDE the source** — a sibling of
  `--source` named `cxoo-reports` (distinctive, so it never collides with an
  existing `reports/` folder in the repo).
- If you **must** keep reports inside the source (e.g. a CI that only uploads
  artifacts from the workspace), the orchestrator **auto-excludes** that directory
  from the tree-scanning engines (SAST `-LocationPathExclude`, KICS `--exclude-paths`,
  2ms `--ignore-pattern`) and prints a warning. SCA (dependency manifests) and
  Container Security (images) don't ingest report files, so they need no exclusion.
- In CI, prefer a path outside the workspace where the platform allows it — e.g.
  GitHub `--output-path "$RUNNER_TEMP/cxoo-reports"` and upload from there.

## Output layout

```
<output-path>/                     # default: a sibling of --source named cxoo-reports (OUTSIDE the tree)
├── run-summary.json               # always written: aggregate verdict + per-engine results
├── sast/sast.{xml,pdf,csv,rtf}
├── sca/sca.{json,sarif,…}
├── iac-security/kics.{json,sarif,html,…}
├── secrets/2ms.{json,sarif,yaml}
└── containers/containers.{json,sarif,…}
```

- `--output-path <dir>` — root directory for all collected reports.
- `--output-name <prefix>` — base name used in `run-summary.json` metadata.
- `run-summary.json` lists, per engine: `ran`, `mode` (native/docker), `route`
  (pass-through / wrapper-side), `childExitCode`, `verdict`, `breaches[]`,
  `counts{}`, the collected `reports[]` paths, and any `warnings[]`.

## Choosing formats: `--report-formats`

`--report-formats <csv>` (default `json,sarif`) is a single unified request applied
to **every** selected engine. For each engine the orchestrator:

1. always emits the **mandatory machine-readable format** it needs for parsing /
   the summary (so gating never breaks regardless of what you request);
2. emits every other requested format the engine **natively supports**;
3. **skips and warns** on formats the engine cannot produce (the warning appears in
   `run-summary.json` and the console — coverage gaps are never silent).

## Per-scanner artifact & format support

| Engine | Tool | Mandatory (always) | Also supported via `--report-formats` | Not supported |
|---|---|---|---|---|
| **SAST** | CxConsolePlugin | `xml` (parsed for counts) | `pdf`, `csv`, `rtf` | json, sarif, html |
| **SCA** | `cx` (ast-cli) | `json` | `sarif`, `sbom`, `pdf`, `markdown`, `html`→summaryHTML, `summary-json`, `gl-sast`, `gl-sca` | csv, xml, rtf |
| **Container Security** | `cx` (ast-cli) | `json` | `sarif`, `sbom`, `pdf`, `markdown`, `html`→summaryHTML, `summary-json`, `gl-sast`, `gl-sca` | csv, xml, rtf |
| **KICS (IaC)** | `kics` | `json` (parsed for counts) | `sarif`, `html`, `pdf`, `csv`, `junit`, `sonarqube`, `cyclonedx`, `asff`, `codeclimate`, `glsast` | xml, rtf |
| **Secrets** | `2ms` | `json` (parsed for counts) | `sarif`, `yaml` | pdf, html, csv, xml, rtf |

Notes:
- **SAST**'s machine format is XML (the CxConsolePlugin does not emit JSON/SARIF);
  the orchestrator parses `sast.xml` for severity counts and the summary.
- **SCA / Containers** report client-side via `cx --report-format <list>`; the
  unified `html` maps to cx's `summaryHTML`. Gating for these engines is native
  (pass-through), so JSON is requested for completeness rather than parsing.
- **KICS / Secrets** gate **wrapper-side**, so their mandatory JSON is parsed for
  per-severity (KICS) / total (2ms) counts.

**Per-scanner overrides:** set a different format set for one engine with
`--<engine>-report-formats <csv>` (e.g. `--sast-report-formats xml,pdf`,
`--sca-report-formats json`). The [configuration builder](../tools/configurator.html)
shows each scanner only the formats it supports (this exact table) and emits the
right per-scanner flags for you.

## Examples

```bash
# Rich artifacts for humans + machines:
cx-onprem-orchestrator run --scanners sast,sca,kics,secrets,containers \
  --report-formats "json,sarif,pdf,html" --output-path ./reports
#   sast      -> sast.xml (mandatory) + sast.pdf            (sarif/html skipped + warned)
#   sca       -> sca.json + sca.sarif + sca.pdf + sca summaryHTML
#   kics      -> kics.json + kics.sarif + kics.pdf + kics.html
#   secrets   -> 2ms.json + 2ms.sarif                        (pdf/html skipped + warned)
#   containers-> containers.json + .sarif + .pdf + summaryHTML
```

Engine-specific native report flags not covered by `--report-formats` can always be
added with the raw passthrough, e.g. `--kics-arg=--report-formats=glsast` or
`--sast-arg=-ReportPDF=/custom/path.pdf`.
