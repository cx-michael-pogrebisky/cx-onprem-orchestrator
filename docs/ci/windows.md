# Windows build agents (all versions)

`cx-onprem-orchestrator` ships a native **Windows** binary, and every v1 engine
tool has a native Windows build — so on Windows you run the engines **directly,
without containers**. The native path is **version-independent**: it works the same
on every in-support Windows release. A container path (the `linux/amd64` fat image)
is *also* available where the OS can host a Linux VM — but it is never required.

> **TL;DR** — On **any** supported Windows (Server 2016 → 2025, Windows 10/11):
> use the **native binary**. Reach for a container only if you specifically want the
> fat image *and* the host can run a Linux VM (WSL2 or Hyper-V) — which rules out
> Windows Server 2016.

## Version support matrix

| Windows version | Native binary (`.exe`) | WSL2 (for container path) | Container path (fat image) | Recommended |
|---|---|---|---|---|
| **Server 2016** (14393) | ✅ (this is the floor) | ❌ none | Hyper-V Linux VM only | **Native** |
| **Server 2019** (17763) | ✅ | ⚠️ manual kernel install only | Hyper-V Linux VM, or WSL2 (manual) | **Native** |
| **Server 2022** (20348) | ✅ | ✅ `wsl --install` | Podman/Docker on WSL2, or Hyper-V VM | Native (container optional) |
| **Server 2025** (26100) | ✅ | ✅ `wsl --install` | Podman/Docker on WSL2, or Hyper-V VM | Native (container optional) |
| **Windows 11** (x64) | ✅ | ✅ | Podman/Docker Desktop on WSL2 | Either |
| **Windows 10 22H2** (19045, x64) | ✅ | ✅ (incl. Home) | Podman/Docker Desktop on WSL2 | Either |
| **Windows 10 1903–21H2** (18362–19044) | ✅ | ✅ | Podman/Docker on WSL2 | Either *(builds EOL — prefer 22H2/11)* |
| **Windows 10 < 1903** (14393–18363) | ✅ | ❌ (below WSL2 floor) | Hyper-V Linux VM (Pro+) only | Native *(builds EOL)* |
| **Windows 11/10 on ARM** | ✅ x64-under-emulation | ✅ | see [ARM](#windows-on-arm) | Native (x64 emulated) |

Notes:
- **Native floor** is the Go toolchain's: *Windows 10 and Windows Server 2016 and
  newer* (Go 1.21 dropped Windows 8.1 / Server 2012 R2). It is **edition-agnostic** —
  Home vs Pro/Enterprise doesn't matter for the native path.
- **WSL2** (what Docker Desktop and Podman use to run a `linux/amd64` image on
  Windows) needs Windows 10 **1903** (build 18362)+ on x64 / **2004**+ on ARM, or
  Windows 11; on Server it's one-command on **2022/2025**, **manual** on **2019**,
  and **unavailable on 2016**. A **Windows container can never run a Linux image**
  (kernels differ), so there is no container path without a Linux VM.
- **Support status:** Windows 10 reached end of mainstream support **2025-10-14**
  (only 22H2 gets ESU). Server 2016 is in extended support to **2027-01**. Prefer
  Server 2022/2025 or Windows 11 for new agents.

## Why Podman, and why containers are limited on Windows

On Windows the standard Docker path is **Docker Desktop**, which under Docker's
Subscription Service Agreement requires a **paid subscription** for organizations
with **250+ employees OR more than US$10M annual revenue** (free for personal use,
education, open source, and smaller businesses). **Podman / Podman Desktop** are
**Apache-2.0 and free at any size**. See
[ci.md → Container runtime](../ci.md#container-runtime--docker-or-podman) for the
full rationale. On Windows, **Podman** also needs a Linux VM (`podman machine`,
WSL2/Hyper-V) — so the same version limits apply, and **Server 2016 can run neither
Docker Desktop nor `podman machine`**. That's the core reason the **native path is
the recommendation on Windows**.

## The native path (works on every version above)

### What to install on the agent

| Component | Source | Notes |
|---|---|---|
| `cx-onprem-orchestrator.exe` | `cx-onprem-orchestrator_<ver>_windows_amd64.zip` (GitHub release) | the orchestrator (`_windows_arm64.zip` also published) |
| **cx** (ast-cli) | `ast-cli_<ver>_windows_x64.zip` → `cx.exe` | SCA + Containers driver |
| **ScaResolver** | `ScaResolver-win64.zip` → `ScaResolver.exe` (+ `Configuration.yml`) | self-contained .NET (no separate runtime) |
| **kics** | `kics_<ver>_windows_amd64.zip` → `kics.exe` (+ `assets\queries`) | IaC |
| **2ms** | `<ver>_windows-amd64.zip` → `2ms.exe` | secrets |
| **CxConsolePlugin** | `CxConsolePlugin-<ver>.zip` (the plugin dir/jar) | SAST |
| **JRE/JDK 11+** | e.g. Eclipse Temurin 17 (x64) | required by CxSAST; not bundled on Windows |
| **Project build toolchain** | Node/npm, Python/pip, Maven, … | for SCA dependency resolution — the project's responsibility, same as any OS |

Put `cx.exe`, `kics.exe`, and `2ms.exe` on `PATH` (so they resolve in **native**
mode), then point the orchestrator at the rest:

```bat
set CXOO_SCA_RESOLVER=C:\cx\sca\ScaResolver.exe
set CXOO_SAST_PATH=C:\cx\CxConsolePlugin
set CXOO_KICS_QUERIES_PATH=C:\cx\kics\assets\queries
```

### How CxSAST runs (no shell script needed)

The orchestrator launches CxSAST as **`java -jar CxConsolePlugin-CLI-*.jar Scan …`**
directly — it does **not** call `runCxConsole.sh`/`.cmd`. `--sast-path` only needs to
point at the plugin **directory** (or the jar). For the JVM, **either** pass the full
path to **`java.exe`** (`--sast-java C:\Java\jdk-17\bin\java.exe`) **or** set
`JAVA_HOME` and put `<JDK>\bin` on `PATH` (the `.exe` suffix is added automatically on
Windows).

### Run it (PowerShell / cmd)

```powershell
C:\cx\cx-onprem-orchestrator.exe run `
  --scanners sast,sca,kics,secrets,containers `
  --sast-team "CxServer/SP" `
  --threshold "sast-critical=1;sca-high=5;secrets-total=1" `
  --sca-resolver C:\cx\sca\ScaResolver.exe `
  --sast-path C:\cx\CxConsolePlugin `
  --sast-java C:\Java\jdk-17\bin\java.exe `
  --output-path reports
```

In Jenkins use a single-line `bat` step (cmd has no line-continuation) — see
[jenkins.md → Windows build agents](jenkins.md#windows-build-agents).

## The container path (Server 2022/2025, Windows 10 1903+/11)

Only where the host can run a Linux VM. One-time setup, then the image runs
identically to any Linux host:

```powershell
# one-time: provision the Linux VM backing Podman
wsl --install                 # if WSL2 isn't present (skip on Server 2016/2019 — see matrix)
podman machine init
podman machine start

# then, per run (Docker is the same command with `docker` instead of `podman`):
podman run --rm -v "${PWD}:/work" -w /work `
  -e CX1_APIKEY -e CXSAST_URL -e CXSAST_USERNAME -e CXSAST_PASSWORD `
  ghcr.io/cx-michael-pogrebisky/cx-onprem-orchestrator:fat `
  run --scanners sast,sca,kics,secrets,containers --threshold "sast-critical=1"
```

Docker Desktop manages its VM automatically (subscription terms apply — see above);
Podman needs the explicit `podman machine init/start`. On **Server 2016** neither
works (no WSL2, `podman machine` unsupported) — use the native path or a separate
Linux agent. On **Server 2019**, WSL2 needs a manual kernel install; a Hyper-V Linux
VM (run Podman/Docker *inside the guest*) is often more reliable.

## Windows on ARM

Run the **x64 builds under the built-in x64 emulation** (Windows 11 on ARM) — this is
the reliable path, since native Windows ARM64 builds are only partial (KICS has one;
Java via the *Microsoft Build of OpenJDK 11* aarch64; cx/ScaResolver/2ms arm64 are not
all available). Our own `cx-onprem-orchestrator_<ver>_windows_arm64.zip` is published,
but mixing it with x64-only engine tools gains little over running the whole x64
toolchain emulated. Windows Server build agents are x64, so this only affects ARM
*workstation* agents.

## Windows gotchas

- **Native mode only** for KICS/2ms on Windows: install their `.exe` on `PATH`. The
  docker-fallback mode builds Linux-style `host:/path:ro` bind mounts that collide
  with Windows `C:\` drive-letter paths and needs a Linux container runtime anyway.
- **Agent as a Windows service:** set `JAVA_HOME`/`PATH` at **machine** scope so the
  service account sees them.
- **Long paths (MAX_PATH):** enable `LongPathsEnabled=1`, set
  `git config --system core.longpaths true`, use a short workspace (e.g. `C:\w\cx`).
- **TLS 1.2** to reach Cx1: a JDK 11+ defaults to it; ensure the agent can route to
  your internal CxSAST host (DNS/firewall/proxy).
- **DAST** stays Docker-only and is out of scope for native Windows (post-v1).
