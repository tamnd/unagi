//go:build linux

package runtime

// resourcePlatformConsts holds the rlimit constants Go's syscall package does not
// define on linux. The values are linux's own from <bits/resource.h>: RLIMIT_RSS
// 5, RLIMIT_NPROC 6, RLIMIT_MEMLOCK 8. test.support reads RLIMIT_NPROC and
// RLIMIT_MEMLOCK when it caps a test's process and locked-memory budgets.
var resourcePlatformConsts = []struct {
	name string
	val  int
}{
	{"RLIMIT_RSS", 5},
	{"RLIMIT_NPROC", 6},
	{"RLIMIT_MEMLOCK", 8},
}
