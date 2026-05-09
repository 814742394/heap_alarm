# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

```bash
# Windows cross-compile (from macOS/Linux)
GOOS=windows GOARCH=amd64 go build -o heap_alarm.exe .

# Local vet
go vet ./...

# Install dependency
go get github.com/shirou/gopsutil/v3/process
```

## Architecture

### Entry point: `main.go` + platform-specific files

The program has three entry points distinguished by Go build tags:

| File | Build Constraint | Role |
|---|---|---|
| `main.go` | all | Shared entry point, config loading, console mode |
| `main_windows.go` | `windows` | Windows service management commands, `isServiceRun()`, `runService()` |
| `main_windows_stub.go` | `!windows` | Stub implementations for non-Windows platforms |

`main.go` dispatches to either `runConsole()` (foreground monitoring) or `runService()` (Windows SCM mode) based on `isServiceRun()` — which detects whether the current process was launched by the Windows Service Controller. This means **the binary itself is the same for both console and service modes**; the mode is determined at runtime.

### Package structure

- **`config/`** — TOML config loading (`go-toml/v2`). Strips UTF-8 BOM. `Validate()` sets `ServiceMode = "console"` as default.
- **`monitor/`** — Process discovery via `gopsutil/v3/process`. `FindAllProcesses` matches process names case-insensitively, stripping `.exe`.
- **`alert/`** — SMTP mailer with cooldown logic. `SendMail` routes to `sendSMTPS` (port 465, TLS dial) or `sendSTARTTLS` (port 587/25, `smtp.SendMail`).
- **`service/`** — Windows service implementation. `service.RunAsService` calls `svc.Run`. Service mode opens its own log file in `Execute()` because `os.Stderr` goes to the Windows Event Log when run under SCM.

### Critical design decisions

**Path resolution**: When running as a Windows service, the working directory is `C:\Windows\System32`, not the executable's directory. All paths (config file via `--config` flag, log file) are resolved relative to the executable's directory using `os.Executable()`.

**Config path**: SCM starts the service with `--config <absolute-path>`. `main.go` parses this flag before loading. Service management commands (`install`/`start`/`stop`/etc.) skip the `--config` flag and load `config.toml` from the executable's directory directly via `configPath()`.

**SMTP ports**: Port 465 uses implicit TLS (SMTPS) — requires `tls.Dial` then SMTP client. Port 587 uses STARTTLS — handled by `smtp.SendMail`. This distinction matters for QQ mail (port 465) vs most other providers (port 587).

**Alert cooldown**: `AlertCooldown.TrySend()` returns `true` only if enough time has passed since the last successful send. `Reset()` is called when memory returns below threshold, so the next spike triggers a fresh alert immediately.

## Service Lifecycle

```cmd
# Install (registers with Windows SCM, passes --config <exe-dir>/config.toml)
heap_alarm.exe install

# Start / stop / restart / remove / status
heap_alarm.exe start
heap_alarm.exe stop
heap_alarm.exe restart
heap_alarm.exe remove
heap_alarm.exe status

# Console mode (foreground monitoring)
heap_alarm.exe
heap_alarm.exe custom.toml   # custom config path
```

All commands must be run as Administrator on Windows Server. Service management commands are handled in `main_windows.go` via `handleServiceCommand()`, which loads config and sets up logging before calling `mgr.Connect()`.

## Key files

- `service/service.go` — `svc.Handler` implementation, monitoring loop, file logging for service mode
- `alert/alert.go` — `SendMail` (SMTPS/STARTTLS routing), `AlertCooldown` (cooldown state machine)
- `monitor/monitor.go` — `FindAllProcesses`, `GetMemoryMB` using gopsutil RSS/WorkingSet
- `config/config.go` — TOML loading, BOM stripping, `Validate()` (defaults `ServiceMode` to `"console"`)
