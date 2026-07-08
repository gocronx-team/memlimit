<h1 align="center">memlimit</h1>

<p align="center">Auto-set Go's <code>GOMEMLIMIT</code> from cgroups — tiny, dependency-free, OOM-safe.</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/gocronx-team/memlimit"><img src="https://pkg.go.dev/badge/github.com/gocronx-team/memlimit.svg?label=Reference" alt="Go Reference"></a>
  <a href="go.mod"><img src="https://img.shields.io/github/go-mod/go-version/gocronx-team/memlimit.svg?label=Go" alt="Go Version"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/gocronx-team/memlimit.svg?label=License" alt="License"></a>
  <a href="go.mod"><img src="https://img.shields.io/badge/deps-0-brightgreen.svg?label=Dependencies" alt="Dependencies"></a>
</p>

---

Sets the soft memory limit (`GOMEMLIMIT`) from a container's cgroup memory
limit, so the garbage collector pushes back **before** the process hits the
ceiling and gets OOM-killed.

## 📖 Why

By default the Go runtime does not know its container's memory limit: the GC
paces itself against **heap growth**, not the cgroup cap. Under memory pressure
a containerized service can overshoot its limit and get OOM-killed and
restarted. Setting `GOMEMLIMIT` to a fraction of the cgroup limit makes the GC
work harder as it nears the ceiling — trading a little CPU for far fewer OOM
kills.

> **Doesn't Go already do this?** As of **Go 1.25** the runtime sets
> `GOMAXPROCS` from the cgroup **CPU** limit automatically — but there is still
> **no** built-in equivalent for `GOMEMLIMIT`. This library fills that gap.

## ✨ Features

- **Zero dependencies**: standard library only.
- **cgroup v2 & v1**: reads `memory.max` (v2) with `memory.limit_in_bytes` (v1) fallback, plus hybrid.
- **Namespace-aware**: resolves the process's own cgroup via `/proc/self/cgroup`, then the mount root.
- **Safe no-op**: no limit, non-Linux, or a too-small limit leaves `GOMEMLIMIT` untouched — the happy path never errors.
- **Death-spiral guard**: refuses to set a pathologically small limit that would trap the GC.
- **Bring your own logger**: silent by default, or pass an `*slog.Logger`.

## 🚀 Quick Start

```shell
go get github.com/gocronx-team/memlimit
```

Call it once, early in `main`:

```go
import (
    "log/slog"

    "github.com/gocronx-team/memlimit"
)

func main() {
    // Set GOMEMLIMIT to 90% of the cgroup memory limit, logging the outcome.
    memlimit.SetFromCgroup(memlimit.WithLogger(slog.Default()))

    // ... start your app ...
}
```

Silent (default):

```go
memlimit.SetFromCgroup()
```

Inspect without changing anything:

```go
bytes, found := memlimit.DetectCgroupLimit()
```

## ⚙️ API

| Symbol | Purpose |
|---|---|
| `SetFromCgroup(opts ...Option) (int64, error)` | Read the cgroup limit and set `GOMEMLIMIT`. Returns bytes set (`0` if unchanged). Errors only on invalid options. |
| `DetectCgroupLimit() (int64, bool)` | Read the raw cgroup limit with no side effects. |
| `WithRatio(r float64)` | Fraction of the cgroup limit to use. Default `0.9`; must be in `(0, 1]`. |
| `WithMinBytes(n int64)` | Skip setting a computed limit below this. Default `16 MiB`. |
| `WithLogger(l *slog.Logger)` | Log the outcome. Silent by default. |

## 🔍 How It Works

- **cgroup v2**: `memory.max` at the process's cgroup path and the mount root (`max` = no limit).
- **cgroup v1**: falls back to `memory.limit_in_bytes` (huge sentinel = no limit).
- **hybrid**: if v2 reports `max` but the v1 memory controller has a real limit, uses the v1 value.
- **no-op**: no limit / non-Linux / below the minimum → `GOMEMLIMIT` left unchanged.

## 📊 Comparison

Benchmarked head-to-head against the reference library it was modeled on
([`KimMachineGun/automemlimit`](https://github.com/KimMachineGun/automemlimit)),
inside a Linux container with a real 256 MiB limit — **both read the identical
limit**, while this implementation is dependency-free and lighter:

| | this library | `automemlimit` v0.7.5 |
|---|---|---|
| Third-party dependencies | **0** | 1 |
| Detection time / op | **~13 µs** | ~40 µs |
| Detection memory / op | **1.7 KB** | 21.6 KB |
| Detection allocs / op | **14** | 119 |

The reference is more feature-rich (dynamic refresh, system-memory fallback,
exotic mount layouts) and more battle-tested; this library is the better fit
for a lightweight, set-once-at-startup helper. Full methodology and an honest
trade-off breakdown: **[COMPARISON.md](COMPARISON.md)**.

## ⚠️ Scope

Targets the common single-container case (mount root + `/proc/self/cgroup`
nesting). It does not parse `mountinfo` for unusual cgroup mount layouts — that
covers the vast majority of Docker/Kubernetes deployments and can be extended
if needed.

## 🙏 Acknowledgements

Thanks to [`KimMachineGun/automemlimit`](https://github.com/KimMachineGun/automemlimit)
— the idea of driving `GOMEMLIMIT` from the cgroup limit, and the behavior this
library models, were learned from it. This is an independent implementation, but
it stands on the approach that project pioneered. Much appreciated. 🙌

## 📄 License

[MIT](LICENSE)
