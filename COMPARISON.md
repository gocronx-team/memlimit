# Comparison with the reference implementation

Reference: [`KimMachineGun/automemlimit`](https://github.com/KimMachineGun/automemlimit)
**v0.7.5** — the well-known library this design was learned from. This is an
independent clean-room implementation (behavior/design referenced, no source
copied).

## How this was measured

Head-to-head, both libraries importing into one test binary, run inside a Linux
container with a **real 256 MiB** cgroup limit:

```shell
docker run --rm --memory=256m -v <project>:/work \
  -w /work/gocronx/memlimit/compare golang:1.23 \
  go test -v -run TestParity -bench=Detect -benchmem ./...
```

See `compare/` for the harness.

## Behavioral parity (correctness)

Under a real 256 MiB limit, both read the identical value:

```
automemlimit: bytes=268435456 found=true
clean-room:   bytes=268435456 found=true
PARITY OK: both read the same cgroup limit of 268435456 bytes
```

The clean-room reader covers cgroup v2 (root + nested path via
`/proc/self/cgroup`), cgroup v1 fallback, and the hybrid case (v2 `max` →
fall back to the v1 memory controller). All verified with Linux fixture tests.

## Measured differences

| Dimension | clean-room `memlimit` | `automemlimit` (reference) |
|---|---|---|
| Third-party dependencies | **0** | 1 (`pbnjay/memory`) |
| Detection: time / op | **~13 µs** | ~40 µs (~3× slower) |
| Detection: memory / op | **1.7 KB** | 21.6 KB (~12× more) |
| Detection: allocs / op | **14** | 119 (~8.5× more) |
| Value read (256 MiB) | 268435456 | 268435456 (identical) |
| GC death-spiral guard | **yes** (`WithMinBytes`, default 16 MiB) | no explicit floor |
| API surface | small: 1 function + 3 options | larger: providers, env, refresh |

(Benchmark: `linux/amd64`, 2 vCPU, real cgroup v2. Detection runs once at
startup, so speed is not operationally critical — but it is an objective,
apples-to-apples measure, and reflects that this implementation does less work.)

## What the reference does that this does not

Being honest about the trade-off — `automemlimit` is more feature-rich and more
battle-tested:

- **Dynamic refresh** (`WithRefreshInterval`) — periodically re-reads the limit.
  This library sets it once at startup.
- **System-memory fallback** (experimental) — derive a limit from total system
  memory when no cgroup limit exists. This library only uses cgroups.
- **Full mountinfo parsing** — handles unusual cgroup mount layouts. This library
  targets the common container case (mount root + `/proc/self/cgroup` nesting).
- **Parent cgroup v2 limits** — v0.7.5 walks up the v2 hierarchy so a limit set
  on a *parent* cgroup (e.g. a Kubernetes pod-level limit above the container) is
  respected. This library checks the process's own cgroup path and the mount
  root, but does not walk intermediate parents. In the common case where the
  limit sits at the container's own cgroup (verified above, both read the same
  256 MiB) the result is identical.
- **Maturity** — years of production use across many projects.

## When to prefer which

- **This library** — for a lightweight, dependency-averse service that sets the
  limit once at startup in a normal container (the gocron case). You get parity
  on the common path, zero dependencies, a smaller/faster code path, and a
  built-in floor against GC death spirals.
- **`automemlimit`** — if you need periodic refresh, a system-memory fallback,
  or coverage of exotic cgroup mount layouts.

For gocron's use case (containerized, set-once-at-startup, "keep it light"),
this implementation is the better fit; it is not a blanket replacement for every
`automemlimit` feature.
