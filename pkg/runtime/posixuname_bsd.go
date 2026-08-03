//go:build darwin || freebsd

package runtime

import "syscall"

// unameFields fills the uname view from sysctl, the way libc's uname does on the
// BSD family (macOS included) where there is no uname syscall: kern.ostype is the
// sysname, kern.hostname the nodename, kern.osrelease the release, kern.version
// the version and hw.machine the machine.
func unameFields() (unameData, error) {
	names := []string{"kern.ostype", "kern.hostname", "kern.osrelease", "kern.version", "hw.machine"}
	vals := make([]string, len(names))
	for i, name := range names {
		v, err := syscall.Sysctl(name)
		if err != nil {
			return unameData{}, err
		}
		vals[i] = v
	}
	return unameData{
		sysname:  vals[0],
		nodename: vals[1],
		release:  vals[2],
		version:  vals[3],
		machine:  vals[4],
	}, nil
}
