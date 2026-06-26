# Windows build agents (native, container-free)

`cx-onprem-orchestrator` ships a native **Windows** binary, and every v1 engine
tool has a native Windows x64 build — so on a Windows agent you run the engines
**directly, without containers**. This is the recommended path on Windows, and
the **only** practical path on **Windows Server 2016**.

## Why not Podman/Docker on Windows Server 2016

A container runs against the host's kernel, so a **Linux** image needs a Linux
kernel — which on Windows means a Linux VM (WSL2 or Hyper-V). On Server 2016:

- **No WSL2.** WSL2 needs Windows build 18362+; Server 2016 is build 14393. WSL
  on Windows Server starts at **Server 2019**; **WSL2 starts at Server 2022**.
  Podman's default `podman machine` backend (WSL2) is therefore unavailable.
- **Hyper-V `podman machine` backend** exists but is new (Podman Desktop 1.13,
  Oct 2024), validated against Windows 10 19043+/Windows 11, and **not supported
  on build 14393**.
- A **Windows container cannot run a Linux image** (kernels are incompatible), and
  our `:fat` image is `linux/amd64`-only. Docker has the same limitation (its old
  LCOW bridge is deprecated and was never available on Server 2016).

So on Server 2016 the docker-vs-podman choice is moot — neither can run the Linux
image. If you specifically want the container workflow, either route those jobs to
a **separate Linux Jenkins agent**, run a **Linux guest VM under Hyper-V** and make
*that* the agent, or **upgrade the host to Server 2022/2025** (which has WSL2, so
Podman's standard backend works). Otherwise, use the native path below.

## What to install on the agent

| Component | Source | Notes |
|---|---|---|
| `cx-onprem-orchestrator.exe` | `cx-onprem-orchestrator_<ver>_windows_amd64.zip` (GitHub release) | the orchestrator |
| **cx** (ast-cli) | `ast-cli_<ver>_windows_x64.zip` → `cx.exe` | SCA + Containers driver |
| **ScaResolver** | `ScaResolver-win64.zip` → `ScaResolver.exe` (+ `Configuration.yml`) | SCA local resolution |
| **kics** | `kics_<ver>_windows_amd64.zip` → `kics.exe` (+ `assets\queries`) | IaC |
| **2ms** | `<ver>_windows-amd64.zip` → `2ms.exe` | secrets |
| **CxConsolePlugin** | `CxConsolePlugin-<ver>.zip` (the plugin dir/jar) | SAST |
| **JRE/JDK 11+** | e.g. Eclipse Temurin 17 (x64) | required by CxSAST; not bundled on Windows |
| **Project build toolchain** | Node/npm, Python/pip, Maven, … | for SCA dependency resolution — the scanned project's responsibility, same as any OS |

Put `cx.exe`, `kics.exe`, and `2ms.exe` on `PATH` (so they resolve in **native**
mode — see the note on docker mode below), then point the orchestrator at the rest:

```bat
set CXOO_SCA_RESOLVER=C:\cx\sca\ScaResolver.exe
set CXOO_SAST_PATH=C:\cx\CxConsolePlugin
set CXOO_KICS_QUERIES_PATH=C:\cx\kics\assets\queries
```

(Or pass `--sca-resolver`, `--sast-path`, `--kics-path`, etc. explicitly.)

## How CxSAST runs (no shell script needed)

The orchestrator launches CxSAST as **`java -jar CxConsolePlugin-CLI-*.jar Scan …`**
directly — it does **not** call `runCxConsole.sh`/`.cmd`. `--sast-path` only needs
to point at the plugin **directory** (or the jar, or either launch script); the jar
is found from there.

For the JVM, **either**:

- pass the full path to **`java.exe`**: `--sast-java C:\Java\jdk-17\bin\java.exe`, **or**
- put `<JDK>\bin` on `PATH` and set `JAVA_HOME` — the orchestrator resolves
  `%JAVA_HOME%\bin\java.exe` (the `.exe` suffix is added automatically on Windows).

## Jenkins pipeline (declarative)

See [jenkins.md → Windows build agents](jenkins.md#windows-build-agents) for the
full `Jenkinsfile`. Key points:

- `agent { label 'windows' }` and `bat` (not `sh`).
- Keep the `cx-onprem-orchestrator.exe run …` command on **one line** — `cmd.exe`
  has no backslash line-continuation.
- Inject secrets with `credentials(...)` / `environment { }` (by env-var name);
  they are masked and never appear on argv.

## Windows gotchas

- **Native mode only** for KICS/2ms on Windows: install their `.exe` on `PATH`.
  The docker-fallback mode builds Linux-style `host:/path:ro` bind mounts that
  collide with Windows `C:\` drive-letter paths, and needs a Linux container
  runtime anyway.
- **Agent as a Windows service:** set `JAVA_HOME`/`PATH` at **machine** scope so
  the service account sees them.
- **Long paths (MAX_PATH):** enable `LongPathsEnabled=1`, set
  `git config --system core.longpaths true`, and use a short workspace
  (e.g. `C:\w\cx`).
- **TLS 1.2** to reach Cx1: a JDK 11+ defaults to it; ensure the agent can route
  to your internal CxSAST host (DNS/firewall/proxy).
- **DAST** stays Docker-only and is out of scope for native Windows (post-v1).
