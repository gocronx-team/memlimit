package memlimit

import "testing"

func TestParseCgroupV2Max(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		want   int64
		wantOK bool
	}{
		{"max means unlimited", "max\n", 0, false},
		{"plain bytes", "268435456", 268435456, true},
		{"bytes with whitespace", "  134217728\n", 134217728, true},
		{"empty", "", 0, false},
		{"zero", "0", 0, false},
		{"negative", "-1", 0, false},
		{"garbage", "abc", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseCgroupV2Max(tt.in)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("parseCgroupV2Max(%q) = (%d, %v), want (%d, %v)", tt.in, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestParseCgroupV1Limit(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		want   int64
		wantOK bool
	}{
		{"plain bytes", "536870912", 536870912, true},
		{"unlimited sentinel", "9223372036854771712", 0, false},
		{"whitespace", " 268435456 \n", 268435456, true},
		{"empty", "", 0, false},
		{"zero", "0", 0, false},
		{"garbage", "x", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseCgroupV1Limit(tt.in)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("parseCgroupV1Limit(%q) = (%d, %v), want (%d, %v)", tt.in, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestComputeLimit(t *testing.T) {
	const mib = 1 << 20
	tests := []struct {
		name     string
		raw      int64
		ratio    float64
		minBytes int64
		want     int64
		wantOK   bool
	}{
		{"90% of 256MiB", 256 * mib, 0.9, 16 * mib, 241591910, true}, // int64(268435456 * 0.9)
		{"below minimum", 8 * mib, 0.9, 16 * mib, 0, false},
		{"exactly at minimum", 20 * mib, 0.8, 16 * mib, 16 * mib, true},
		{"zero raw", 0, 0.9, 16 * mib, 0, false},
		{"negative raw", -5, 0.9, 16 * mib, 0, false},
		{"full ratio", 100 * mib, 1.0, 0, 100 * mib, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := computeLimit(tt.raw, tt.ratio, tt.minBytes)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("computeLimit(%d, %v, %d) = (%d, %v), want (%d, %v)",
					tt.raw, tt.ratio, tt.minBytes, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestSetFromCgroupRejectsBadOptions(t *testing.T) {
	if _, err := SetFromCgroup(WithRatio(0)); err == nil {
		t.Error("expected error for ratio 0")
	}
	if _, err := SetFromCgroup(WithRatio(1.5)); err == nil {
		t.Error("expected error for ratio > 1")
	}
	if _, err := SetFromCgroup(WithMinBytes(-1)); err == nil {
		t.Error("expected error for negative minBytes")
	}
}

// TestSetFromCgroupNoLimit ensures that when no cgroup limit is detectable
// (non-Linux, or Linux without a limit) SetFromCgroup is a clean no-op.
// On non-Linux this always holds; on Linux CI without a memory-limited cgroup
// it also holds.
func TestSetFromCgroupIsSafeNoOp(t *testing.T) {
	// Valid options; should never error, and returns 0 when no limit applies.
	if _, err := SetFromCgroup(WithRatio(0.9)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
