//go:build !linux

package memlimit

// readCgroupMemoryLimit is a no-op on non-Linux platforms, where cgroup memory
// limits do not apply. SetFromCgroup therefore leaves GOMEMLIMIT unchanged.
func readCgroupMemoryLimit() (bytes int64, source string, found bool) {
	return 0, "", false
}
