# SCA (SCA Resolver mode)

## SCA is **always** run through SCA Resolver

`cx-onprem-orchestrator` runs Cx1 SCA **exclusively in SCA Resolver mode** — it
never uploads source for cloud-side resolution. The `cx` invocation always
includes `--sca-resolver <path>`, and the SCA engine is **unavailable without it**:
omitting `--sca-resolver` (and with no `CXOO_SCA_RESOLVER` default) fails the SCA
prerequisite check with exit code `31`:

```
SCA Resolver mode requires --sca-resolver <path to ScaResolver>
```

**Prerequisites** (enforced by `Available()`):
- the `cx` CLI (default `cx`, or `--sca-path`);
- the **ScaResolver** binary via `--sca-resolver <path>` (the fat image sets
  `CXOO_SCA_RESOLVER=/opt/sca/ScaResolver`, so the flag is optional there);
- a `Configuration.yml` next to the ScaResolver binary;
- Cx1 auth (API key or client-credentials — see [authentication.md](authentication.md));
- the scanned project's **language build tools / package managers** installed
  (node/npm, python/pip, maven, gradle, nuget, go, …) and a **local folder**
  source (SCA Resolver does not accept a zip or a URL).

> ⚠️ **The fat image does NOT bundle project build toolchains.** SCA Resolver
> performs *local* dependency resolution, so it needs the scanned project's own
> package managers present in the **execution environment** — these vary per
> project and are your responsibility, not the orchestrator's. Provide them in
> your CI before the scan step (e.g. `actions/setup-node` + `actions/setup-python`,
> or an `apt-get`/`apk add` step), or build a downstream image `FROM` the fat
> image with the toolchain added. Only the engine tools and a JRE (the CxSAST
> runtime) are bundled.

## Passing arguments — two separate channels

| Flag | Forwarded to | Becomes |
|---|---|---|
| `--sca-resolver-arg=<token>` | the **ScaResolver** binary | appended into `cx --sca-resolver-params "…"` |
| `--sca-arg=<token>` | the **cx** CLI (`cx scan create`) | appended to the cx argv |

Both are **repeatable** and **`=`-bound** (so leading-dash values are safe).

### ScaResolver arguments → `--sca-resolver-arg`

Each occurrence contributes **one token** to the single string `cx` passes to
ScaResolver via `--sca-resolver-params`. For a ScaResolver flag that takes a
**separate value**, pass two occurrences (flag, then value):

```bash
cx-onprem-orchestrator run --scanners sca --source . \
  --sca-resolver /opt/sca/ScaResolver \
  --sca-resolver-arg=--log-level --sca-resolver-arg=Debug \          # --log-level Debug
  --sca-resolver-arg='--excludes **/test/**,**/vendor/**' \          # value with spaces/dashes
  --sca-resolver-arg=--scan-containers --sca-resolver-arg=true        # --scan-containers true
```

These reach the ScaResolver binary; the orchestrator joins them, in order, into:

```
cx scan create … --sca-resolver /opt/sca/ScaResolver \
   --sca-resolver-params "--log-level Debug --excludes **/test/**,**/vendor/** --scan-containers true" …
```

Common ScaResolver flags you might pass this way: `--log-level <level>`,
`--excludes <glob,glob>`, `-i/--ignore-dev-dependencies`,
`--ignore-test-dependencies`, `--scan-containers true --images <img>`,
`--private-dependency-type/-name/-version`, `--sbom-first`. Run
`ScaResolver offline --help` for the full list.

### cx (Checkmarx One) arguments → `--sca-arg`

Use `--sca-arg` for flags of the `cx scan create` command itself (not the resolver):

```bash
  --sca-arg=--project-tags=team:appsec \
  --sca-arg=--scan-info-format=json
```

### Putting it together

```bash
cx-onprem-orchestrator run \
  --scanners sca --source . --project-name my-app --branch main \
  --threshold "sca-high=5;sca-critical=1" \
  --sca-filter "(?i).*(test|mock).*" \                 # cx SCA result filter (regex, Tier-A)
  --sca-resolver /opt/sca/ScaResolver \
  --sca-resolver-arg=--log-level --sca-resolver-arg=Debug \
  --sca-resolver-arg='--excludes **/generated/**' \
  --sca-arg=--project-tags=team:appsec \
  --cx-apikey-env CX1_APIKEY
```

Run `cx-onprem-orchestrator validate …` (or `run --dry-run`) to print the exact,
fully-resolved `cx` command — including the assembled `--sca-resolver-params` — with
secrets redacted, before any scan launches.

## Troubleshooting: SCA export is slow or times out

**Symptom.** The SCA engine appears to hang for 10–20 minutes after the scan
shows `Completed`, printing `SCA Export Status is: Pending` repeatedly, and may
end with `Failed listing results: export generating failed - Timed out after 5
minutes`.

**What's happening.** An SCA run has three phases:

1. **Local resolution** (`ScaResolver`) — usually seconds to ~2 min.
2. **Cloud scan** on Cx1 — typically a few minutes.
3. **Results/report export** — Cx1 generates the risk report via its export
   service; `cx` polls it until ready.

The slowness is **phase 3 on the Cx1 side**. Critically, `cx` retrieves the SCA
results for **both** the report **and** the `--threshold` gate from this same
export service — so when the export service stalls, neither the report nor the
gate can complete, regardless of `--report-format`. `cx` has a fixed ~5-minute
export timeout (not configurable) and then exits non-zero.

**How the orchestrator handles it.** A stalled/failed export is reported as an
**engine failure** for SCA (with an explanatory warning in `run-summary.json`) —
never as a false threshold breach. The other engines are unaffected (the
report-collection barrier means their reports are still written).

**Mitigations.**

- **Bound the wait** with `--sca-timeout <dur>` (e.g. `--sca-timeout 8m`) so a
  wedged export is abandoned predictably instead of burning ~20 min per run.
- **Retry** — export stalls are often transient backend load. If they persist
  for a tenant, it's a Checkmarx export-service condition to raise with support
  (reproducible directly with `cx results show --scan-id <id> --report-format
  json`, independent of this tool).
- This is **not** caused by local resolution or by missing build tools; verify
  separately with `ScaResolver offline -s <dir> -n <name> -r out.json`, which
  exercises only phase 1.
