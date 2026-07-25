//go:build darwin

package runtime

// resourcePlatformConsts holds the rlimit constants Go's syscall package does not
// define on darwin. The values are darwin's own from <sys/resource.h>: RLIMIT_RSS
// is an alias for RLIMIT_AS (5), and darwin's BSD lineage numbers RLIMIT_MEMLOCK
// 6 and RLIMIT_NPROC 7. test.support reads RLIMIT_NPROC and RLIMIT_MEMLOCK when
// it caps a test's process and locked-memory budgets.
var resourcePlatformConsts = []struct {
	name string
	val  int
}{
	{"RLIMIT_RSS", 5},
	{"RLIMIT_MEMLOCK", 6},
	{"RLIMIT_NPROC", 7},
}
