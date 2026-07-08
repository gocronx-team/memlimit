//go:build linux

package memlimit

import (
	"os"
	"path/filepath"
	"strings"
)

// Filesystem locations. They are variables so tests can point them at fixtures.
var (
	// cgroupV2Root is the cgroup v2 mount point.
	cgroupV2Root = "/sys/fs/cgroup"
	// procCgroupPath describes the current process's cgroup membership.
	procCgroupPath = "/proc/self/cgroup"
	// cgroupV1MemLimitPath is the cgroup v1 memory limit file.
	cgroupV1MemLimitPath = "/sys/fs/cgroup/memory/memory.limit_in_bytes"
)

// readCgroupMemoryLimit reads the effective cgroup memory limit on Linux,
// preferring cgroup v2 and falling back to v1. The bool reports whether a real
// (finite) limit was found.
func readCgroupMemoryLimit() (bytes int64, source string, found bool) {
	if n, ok := readCgroupV2(); ok {
		return n, "cgroup2:memory.max", true
	}
	if b, err := os.ReadFile(cgroupV1MemLimitPath); err == nil {
		if n, ok := parseCgroupV1Limit(string(b)); ok {
			return n, "cgroup1:memory.limit_in_bytes", true
		}
	}
	return 0, "", false
}

// readCgroupV2 resolves the process's own cgroup v2 memory.max, then falls back
// to the mount root. Under cgroup namespaces (the modern Docker/Kubernetes
// default) the process path is "/" so the first candidate is already the root;
// with a host cgroup namespace the nested path is the meaningful one.
func readCgroupV2() (int64, bool) {
	for _, p := range cgroupV2Candidates() {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		// First existing memory.max wins: a finite value is the limit, "max"
		// at this level means no limit is enforced here.
		return parseCgroupV2Max(string(b))
	}
	return 0, false
}

// cgroupV2Candidates lists memory.max paths to try, most specific first:
// the process's own cgroup path (from /proc/self/cgroup) then the mount root.
func cgroupV2Candidates() []string {
	root := cgroupV2Root
	candidates := make([]string, 0, 2)
	if rel := procV2RelPath(); rel != "" && rel != "/" {
		candidates = append(candidates, filepath.Join(root, rel, "memory.max"))
	}
	candidates = append(candidates, filepath.Join(root, "memory.max"))
	return candidates
}

// procV2RelPath extracts the cgroup v2 relative path from /proc/self/cgroup.
// The v2 entry is the line beginning with "0::". Returns "" if not found.
func procV2RelPath() string {
	b, err := os.ReadFile(procCgroupPath)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		// Format: hierarchy-ID:controller-list:cgroup-path; v2 is "0::<path>".
		if rest, ok := strings.CutPrefix(line, "0::"); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}
