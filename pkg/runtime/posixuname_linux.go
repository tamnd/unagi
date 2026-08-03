//go:build linux

package runtime

import "syscall"

// unameFields fills the uname view from the Linux utsname struct the uname(2)
// syscall populates. The struct's char arrays are int8 on some arches and uint8
// on others, so charArrayToString takes either and stops at the NUL terminator.
func unameFields() (unameData, error) {
	var u syscall.Utsname
	if err := syscall.Uname(&u); err != nil {
		return unameData{}, err
	}
	return unameData{
		sysname:  charArrayToString(u.Sysname[:]),
		nodename: charArrayToString(u.Nodename[:]),
		release:  charArrayToString(u.Release[:]),
		version:  charArrayToString(u.Version[:]),
		machine:  charArrayToString(u.Machine[:]),
	}, nil
}

// charArrayToString turns a NUL-terminated C char array into a Go string,
// accepting the int8 or uint8 element type Go's Utsname uses per arch.
func charArrayToString[T int8 | byte](a []T) string {
	n := 0
	for n < len(a) && a[n] != 0 {
		n++
	}
	b := make([]byte, n)
	for i := 0; i < n; i++ {
		b[i] = byte(a[i])
	}
	return string(b)
}
