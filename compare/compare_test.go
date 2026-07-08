// Package compare runs the clean-room memlimit implementation head-to-head
// against the reference (KimMachineGun/automemlimit) on the same host.
//
// TestParity asserts both agree on whether a cgroup memory limit exists and,
// when it does, on the exact byte value. Run it under a real limit to make it
// meaningful, e.g.:
//
//	docker run --rm --memory=256m -v <project>:/work \
//	  -w /work/gocronx/memlimit/compare golang:1.23 go test -v ./...
package compare

import (
	"testing"

	am "github.com/KimMachineGun/automemlimit/memlimit"
	mine "github.com/gocronx-team/memlimit"
)

func TestParity(t *testing.T) {
	amBytes, amErr := am.FromCgroup()
	amFound := amErr == nil
	myBytes, myFound := mine.DetectCgroupLimit()

	t.Logf("automemlimit: bytes=%d found=%v err=%v", amBytes, amFound, amErr)
	t.Logf("clean-room:   bytes=%d found=%v", myBytes, myFound)

	if amFound != myFound {
		t.Fatalf("disagreement on limit presence: automemlimit=%v clean-room=%v", amFound, myFound)
	}
	if amFound && int64(amBytes) != myBytes {
		t.Fatalf("disagreement on limit value: automemlimit=%d clean-room=%d", amBytes, myBytes)
	}
	if amFound {
		t.Logf("PARITY OK: both read the same cgroup limit of %d bytes", myBytes)
	} else {
		t.Log("PARITY OK: both report no cgroup limit on this host")
	}
}

func BenchmarkDetect_CleanRoom(b *testing.B) {
	for i := 0; i < b.N; i++ {
		mine.DetectCgroupLimit()
	}
}

func BenchmarkDetect_Automemlimit(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = am.FromCgroup()
	}
}
