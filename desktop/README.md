# Desktop

The desktop build. A Tauri window around the same engine the command line uses.

## How it fits together

```
  ui/                     HTML, CSS and JavaScript. No framework, no build step.
   │  invoke('run_diagnostic')
   ▼
  src-tauri/src/main.rs   Rust. Spawns the sidecar, parses its output.
   │  wifitest --json --no-history
   ▼
  binaries/wifitest-<target-triple>   the engine, built from ../../cmd/wifitest
```

The window measures nothing. Every figure it shows was printed by the engine,
which is the same program `go build ./cmd/wifitest` produces — bundled here as a
sidecar and invoked per run. The boundary is a process, not a library binding,
which is why the engine needs no knowledge that a desktop build exists.

The engine's JSON is a published contract with a `schema_version`; this window is
its first consumer other than a shell script.

## Building

Requires the Go toolchain, Rust, and on Windows the WebView2 runtime (present by
default on Windows 11).

Build the engine into the sidecar slot first — the file name must carry the
target triple, which is what Tauri looks for:

```
go build -o desktop/src-tauri/binaries/wifitest-$(rustc -vV | sed -n 's/^host: //p')$(test "$OS" = Windows_NT && echo .exe) ./cmd/wifitest
```

On Windows PowerShell:

```
go build -o "desktop\src-tauri\binaries\wifitest-x86_64-pc-windows-msvc.exe" .\cmd\wifitest
```

Then, from `desktop/src-tauri`:

```
cargo tauri dev      # run it
cargo tauri build    # produce an installer
```

The sidecar binary is not committed. It is the engine compiled from source that
is already in this repository, and storing it would keep the same program twice.

## What the window shows

The path, left to right, in the order packets travel: your network, your
provider, the wider internet. Selecting a segment shows its round-trip time,
jitter, loss, the port that answered, and what a fault there would mean.

Throughput reads `not measured` until the built-in endpoint list is filled,
which is waiting on a terms-of-service review of each candidate. The layered
diagnostics do not depend on it.

The status bar carries the schema version and the engine's exit code, and the
raw payload is one click away — both because the audience includes people who
will script this, and because it is the plainest evidence that the window is
rendering real output rather than decoration.
