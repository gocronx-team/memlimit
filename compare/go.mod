// This is a nested module used only to compare against the reference
// implementation. It is NOT part of the published github.com/gocronx-team/memlimit
// module, so the library itself stays dependency-free.
module github.com/gocronx-team/memlimit/compare

go 1.24

require (
	github.com/KimMachineGun/automemlimit v0.7.5
	github.com/gocronx-team/memlimit v0.0.0
)

require github.com/pbnjay/memory v0.0.0-20210728143218-7b4eea64cf58 // indirect

replace github.com/gocronx-team/memlimit => ../
