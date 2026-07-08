// Command example demonstrates and verifies the end-to-end effect of memlimit:
// it applies GOMEMLIMIT from the cgroup, then reads the runtime's current value
// back via debug.SetMemoryLimit(-1) to confirm it was actually applied.
//
//	# no limit -> no-op, GOMEMLIMIT stays at the default (max int64)
//	go run .
//
//	# real 256 MiB limit -> GOMEMLIMIT set to 90% (241591910)
//	docker run --rm --memory=256m -v <project>:/work \
//	  -w /work/gocronx/memlimit golang:1.23 go run ./example
package main

import (
	"fmt"
	"log/slog"
	"runtime/debug"

	"github.com/gocronx-team/memlimit"
)

func main() {
	before := debug.SetMemoryLimit(-1) // read-only: -1 returns the current limit

	set, err := memlimit.SetFromCgroup(memlimit.WithLogger(slog.Default()))

	after := debug.SetMemoryLimit(-1)

	fmt.Println("---- memlimit verification ----")
	fmt.Printf("GOMEMLIMIT before : %d\n", before)
	fmt.Printf("SetFromCgroup ->  : %d bytes (err=%v)\n", set, err)
	fmt.Printf("GOMEMLIMIT after  : %d\n", after)

	switch {
	case set == 0:
		fmt.Println("result: no cgroup limit detected — GOMEMLIMIT left unchanged (no-op)")
	case after == set:
		fmt.Printf("result: OK — runtime GOMEMLIMIT now matches the value we set (%d)\n", set)
	default:
		fmt.Printf("result: MISMATCH — set %d but runtime reports %d\n", set, after)
	}
}
