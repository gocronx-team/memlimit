// Package memlimit sets Go's soft memory limit (GOMEMLIMIT) from the container's
// cgroup memory limit, so the garbage collector applies back-pressure before the
// process hits the cgroup ceiling and gets OOM-killed.
//
// By default Go does not know its container's memory limit: the GC paces itself
// against heap growth, not against the cgroup cap. Under memory pressure that
// leads to OOM kills and restarts. Calling SetFromCgroup once at startup teaches
// the runtime the limit (defaulting to 90% of it, leaving headroom for
// non-heap allocations the GC cannot reclaim).
//
// It is a safe no-op when there is no cgroup memory limit, on non-Linux
// platforms, or when the limit is too small to be sane.
package memlimit

import (
	"fmt"
	"log/slog"
	"runtime/debug"
	"strconv"
	"strings"
)

const (
	// defaultRatio is the fraction of the cgroup limit used for GOMEMLIMIT.
	// Headroom is left for stacks, goroutine metadata and other non-heap memory
	// the GC target does not account for. 0.9 is a common, conservative choice.
	defaultRatio = 0.9

	// defaultMinBytes guards against pathologically small limits. Setting
	// GOMEMLIMIT very low forces the GC into a death spiral (it runs constantly
	// yet can never get under the target). Below this, leave GOMEMLIMIT alone.
	defaultMinBytes int64 = 16 << 20 // 16 MiB

	// v1Unlimited is the threshold above which a cgroup v1 memory limit is
	// treated as "no limit". v1 reports unlimited as a huge sentinel value;
	// any limit beyond a few exbibytes is not a real container cap.
	v1Unlimited int64 = 1 << 62
)

// config holds resolved options for SetFromCgroup.
type config struct {
	ratio    float64
	minBytes int64
	logger   *slog.Logger
}

// Option customizes SetFromCgroup.
type Option func(*config)

// WithRatio sets the fraction of the cgroup limit used for GOMEMLIMIT.
// r must be in (0, 1]; the default is 0.9.
func WithRatio(r float64) Option {
	return func(c *config) { c.ratio = r }
}

// WithMinBytes sets the smallest computed limit worth applying. If the computed
// value is below this, GOMEMLIMIT is left unchanged. The default is 16 MiB.
func WithMinBytes(n int64) Option {
	return func(c *config) { c.minBytes = n }
}

// WithLogger enables logging of the outcome via the given slog.Logger.
// By default nothing is logged.
func WithLogger(l *slog.Logger) Option {
	return func(c *config) { c.logger = l }
}

func (c *config) log(msg string, args ...any) {
	if c.logger != nil {
		c.logger.Info(msg, args...)
	}
}

// SetFromCgroup reads the current cgroup memory limit and, if found, sets
// GOMEMLIMIT to that limit multiplied by the configured ratio.
//
// It returns the number of bytes GOMEMLIMIT was set to, or 0 if it was left
// unchanged (no cgroup limit, non-Linux, unlimited, or below the minimum).
// A non-nil error is returned only for invalid options.
func SetFromCgroup(opts ...Option) (int64, error) {
	c := config{ratio: defaultRatio, minBytes: defaultMinBytes}
	for _, o := range opts {
		o(&c)
	}
	if c.ratio <= 0 || c.ratio > 1 {
		return 0, fmt.Errorf("memlimit: ratio must be in (0, 1], got %v", c.ratio)
	}
	if c.minBytes < 0 {
		return 0, fmt.Errorf("memlimit: minBytes must be >= 0, got %d", c.minBytes)
	}

	raw, source, found := readCgroupMemoryLimit()
	if !found {
		c.log("memlimit: no cgroup memory limit detected, GOMEMLIMIT left unchanged")
		return 0, nil
	}

	limit, ok := computeLimit(raw, c.ratio, c.minBytes)
	if !ok {
		c.log("memlimit: cgroup limit below minimum, GOMEMLIMIT left unchanged",
			"cgroup_bytes", raw, "min_bytes", c.minBytes, "source", source)
		return 0, nil
	}

	debug.SetMemoryLimit(limit)
	c.log("memlimit: GOMEMLIMIT set from cgroup",
		"gomemlimit_bytes", limit, "cgroup_bytes", raw, "ratio", c.ratio, "source", source)
	return limit, nil
}

// DetectCgroupLimit returns the raw cgroup memory limit in bytes (before any
// ratio is applied) and whether a finite limit was found. It performs no
// side effects, so it is useful for inspecting or comparing what SetFromCgroup
// would act on. Returns (0, false) on non-Linux or when there is no limit.
func DetectCgroupLimit() (bytes int64, found bool) {
	b, _, ok := readCgroupMemoryLimit()
	return b, ok
}

// computeLimit applies the ratio to a raw cgroup limit and enforces the minimum.
// It returns the value to set and whether it is worth setting.
func computeLimit(raw int64, ratio float64, minBytes int64) (int64, bool) {
	if raw <= 0 {
		return 0, false
	}
	limit := int64(float64(raw) * ratio)
	if limit < minBytes {
		return 0, false
	}
	return limit, true
}

// parseCgroupV2Max parses the contents of a cgroup v2 memory.max file.
// The literal "max" means no limit. Returns (bytes, ok).
func parseCgroupV2Max(content string) (int64, bool) {
	s := strings.TrimSpace(content)
	if s == "" || s == "max" {
		return 0, false
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

// parseCgroupV1Limit parses the contents of a cgroup v1 memory.limit_in_bytes
// file. v1 reports "unlimited" as a very large sentinel value. Returns (bytes, ok).
func parseCgroupV1Limit(content string) (int64, bool) {
	s := strings.TrimSpace(content)
	if s == "" {
		return 0, false
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 || n >= v1Unlimited {
		return 0, false
	}
	return n, true
}
