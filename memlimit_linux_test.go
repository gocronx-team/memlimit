//go:build linux

package memlimit

import (
	"os"
	"path/filepath"
	"testing"
)

// setPaths overrides the cgroup/proc locations for a test and restores them after.
func setPaths(t *testing.T, v2Root, proc, v1 string) {
	t.Helper()
	oR, oP, oV1 := cgroupV2Root, procCgroupPath, cgroupV1MemLimitPath
	cgroupV2Root, procCgroupPath, cgroupV1MemLimitPath = v2Root, proc, v1
	t.Cleanup(func() { cgroupV2Root, procCgroupPath, cgroupV1MemLimitPath = oR, oP, oV1 })
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// cgroup namespace = private (modern default): process path is "/", limit sits
// at the mount root.
func TestReadV2_RootPrivateNamespace(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "memory.max"), "268435456\n")
	proc := filepath.Join(t.TempDir(), "cgroup")
	write(t, proc, "0::/\n")
	setPaths(t, root, proc, "/nonexistent/v1")

	b, source, found := readCgroupMemoryLimit()
	if !found || b != 268435456 || source != "cgroup2:memory.max" {
		t.Fatalf("got (%d, %q, %v), want (268435456, cgroup2:memory.max, true)", b, source, found)
	}
}

// cgroup namespace = host: limit sits at the nested path from /proc/self/cgroup.
func TestReadV2_NestedHostNamespace(t *testing.T) {
	root := t.TempDir()
	rel := "/kubepods/podabc/container123"
	write(t, filepath.Join(root, rel, "memory.max"), "134217728\n")
	// A different value at the root proves we picked the nested (specific) one.
	write(t, filepath.Join(root, "memory.max"), "999999999\n")
	proc := filepath.Join(t.TempDir(), "cgroup")
	write(t, proc, "0::"+rel+"\n")
	setPaths(t, root, proc, "/nonexistent/v1")

	b, _, found := readCgroupMemoryLimit()
	if !found || b != 134217728 {
		t.Fatalf("got (%d, %v), want nested 134217728", b, found)
	}
}

// Unified v2 with no limit and no v1 controller (the modern default) -> no limit.
func TestReadV2_UnlimitedNoV1(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "memory.max"), "max\n")
	proc := filepath.Join(t.TempDir(), "cgroup")
	write(t, proc, "0::/\n")
	setPaths(t, root, proc, "/nonexistent/v1")

	if _, _, found := readCgroupMemoryLimit(); found {
		t.Fatal("v2 'max' with no v1 controller must report no limit")
	}
}

// Hybrid cgroups: v2 mounted but the memory controller (and its limit) lives on
// v1. When v2 memory.max is "max", the real limit is v1's, so we use it.
func TestReadHybrid_V2MaxFallsBackToV1(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "memory.max"), "max\n")
	proc := filepath.Join(t.TempDir(), "cgroup")
	write(t, proc, "0::/\n")
	v1 := filepath.Join(t.TempDir(), "limit")
	write(t, v1, "536870912\n")
	setPaths(t, root, proc, v1)

	b, source, found := readCgroupMemoryLimit()
	if !found || b != 536870912 || source != "cgroup1:memory.limit_in_bytes" {
		t.Fatalf("got (%d, %q, %v), want v1 fallback 536870912", b, source, found)
	}
}

func TestReadV1_Fallback(t *testing.T) {
	v1 := filepath.Join(t.TempDir(), "limit")
	write(t, v1, "536870912\n")
	setPaths(t, "/nonexistent/v2root", "/nonexistent/proc", v1)

	b, source, found := readCgroupMemoryLimit()
	if !found || b != 536870912 || source != "cgroup1:memory.limit_in_bytes" {
		t.Fatalf("got (%d, %q, %v), want (536870912, cgroup1:..., true)", b, source, found)
	}
}

func TestRead_NoneFound(t *testing.T) {
	setPaths(t, "/nonexistent/v2root", "/nonexistent/proc", "/nonexistent/v1")
	if _, _, found := readCgroupMemoryLimit(); found {
		t.Fatal("expected no limit when nothing exists")
	}
}
