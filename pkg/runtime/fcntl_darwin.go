//go:build darwin

package runtime

// fcntlPlatformConsts holds fcntl commands that exist only on darwin. The common
// F_* and LOCK_* commands are in the shared table; darwin's extra commands
// (F_FULLFSYNC and friends) are not reached by any stdlib module the floor runs,
// so the platform table is empty here. It exists so the shared init can add
// host-specific commands uniformly, the way linux adds the pipe-size commands.
var fcntlPlatformConsts = []struct {
	name string
	val  int
}{}
