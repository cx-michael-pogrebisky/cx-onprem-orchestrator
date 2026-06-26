# AppVeyor

`cx-onprem-orchestrator` auto-detects AppVeyor (`APPVEYOR=True`), reading
branch/commit/repo from `APPVEYOR_REPO_BRANCH`/`APPVEYOR_REPO_COMMIT`/
`APPVEYOR_REPO_NAME` and the workspace from `APPVEYOR_BUILD_FOLDER`.

> **Docker or Podman?** The fat image runs identically under either — see
> [Container runtime](../ci.md#container-runtime--docker-or-podman). AppVeyor is
> Windows-first, so run the fat image on a Linux **`image: Ubuntu`** worker (Docker
> is bundled); if you prefer Podman, install it in that worker. Windows images run
> the native binary, not the fat image.

> **Use the fat image** (all engine tools bundled, digest-pinned). Installing them
> yourself is a [significantly less recommended path](../ci.md#image-choice).

```yaml
# appveyor.yml  (Linux image, Docker available)
image: Ubuntu2204
services:
  - docker
build_script:
  - sh: >
      docker run --rm -v "$APPVEYOR_BUILD_FOLDER":/work -w /work
      -e CX1_APIKEY -e CXSAST_URL -e CXSAST_USERNAME -e CXSAST_PASSWORD
      ghcr.io/cx-michael-pogrebisky/cx-onprem-orchestrator:fat run
      --scanners sast,sca,kics,secrets
      --threshold "sast-critical=1;sca-high=5;secrets-total=1"
      --output-path /work/reports
# Define CX1_APIKEY / CXSAST_* as (encrypted) AppVeyor environment variables.
```

## Windows agents

AppVeyor is **Windows-first**: its default workers are Windows (`image: Visual
Studio 2022`), and the `linux/amd64` fat image needs an explicit **Linux image
label** (`image: Ubuntu`, as above). On a Windows worker the image won't run — use
the **native Windows binary**; see [windows.md](./windows.md) for full setup.

```yaml
# appveyor.yml  (hosted Windows worker — native binary)
image: Visual Studio 2022
build_script:
  - cmd: >
      C:\cx\cx-onprem-orchestrator.exe run
      --scanners sast,sca,kics,secrets
      --threshold "sast-critical=1;sca-high=5;secrets-total=1"
      --sast-java C:\Java\jdk-17\bin\java.exe
      --sca-resolver C:\cx\sca\ScaResolver.exe
      --sast-path C:\cx\CxConsolePlugin
      --output-path reports
# Inject CX1_APIKEY / CXSAST_* as (encrypted) AppVeyor environment variables — never on argv.
```
