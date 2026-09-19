# WiFi Speed Test

A cross-platform network diagnostic tool. It does not just report how fast your
connection is — it tells you which segment of the path is responsible when
something is wrong.

> Status: early development. The command line interface is the first target;
> desktop and Android builds follow.

## What it measures

- Download and upload throughput, averaged across the endpoints you supply
- Latency, jitter and packet loss
- DNS resolution time
- **Layered diagnostics** — latency, jitter and loss are measured separately at
  three points along the path, so a problem can be attributed rather than merely
  observed:

  | Layer | Target | What it tells you |
  | --- | --- | --- |
  | Local gateway | Auto-detected default gateway | A fault here is inside your own network or Wi-Fi |
  | Regional egress | Auto-detected system DNS resolver | A fault here is inside your ISP |
  | International | Public DNS address | A fault only here points at international capacity |

## Modes

`wifitest` runs a single diagnostic pass and exits. `wifitest --watch` repeats at
a configurable interval, accumulating trend data, and can push each report to a
webhook you supply.

## Design principles

**No cloud services.** This project operates no servers of its own. There is no
account, no database, no sync and no telemetry. Results and history stay on your
device.

**You control where your traffic goes.** Throughput is measured only against
endpoints you name, with `--endpoint` or in the configuration file. The built-in
list ships empty on purpose: a review of the public candidates cleared none of
them for automated use at the rate this tool would call them, and a default
nobody vetted is worse than no default at all. The first two diagnostic layers
target your own gateway and resolver; only the third reaches off your network,
to a documented and overridable default. Nothing is reported to the project.

**Credentials never live in source.** Webhook URLs belong in your local
configuration file, which is excluded from version control.

## Platforms

| Platform | CLI | Desktop | Native app |
| --- | --- | --- | --- |
| Windows | planned | planned | — |
| Linux | planned | planned | — |
| macOS | planned | planned | — |
| Android (Termux) | planned | — | — |
| Android | — | — | planned |

## Building

Requires Go 1.26 or newer.

```
go build ./cmd/wifitest
```

## License

MIT. See [LICENSE](LICENSE).

This project depends on `golang.org/x/sys` and `golang.org/x/net`, both
published by the Go authors under the BSD-3-Clause license.

Copyright (c) 2026 SpaceSquare
