//go:build linux

package runtime

import "syscall"

// posixStatExtraNames is empty on Linux: the plain fstat/stat path fills no
// fields past the common set. Linux CPython's st_flags/st_birthtime come from
// statx, which this skeleton does not call, so they are left off rather than
// reported with zero values that would not match the host.
var posixStatExtraNames []string

// statNormalize reads a Linux syscall.Stat_t into the host-independent carrier.
// Linux names the time fields Atim/Mtim/Ctim.
func statNormalize(st *syscall.Stat_t) statNormal {
	return statNormal{
		mode:      int64(st.Mode),
		ino:       int64(st.Ino),
		dev:       int64(st.Dev),
		nlink:     int64(st.Nlink),
		uid:       int64(st.Uid),
		gid:       int64(st.Gid),
		size:      int64(st.Size),
		atimeSec:  int64(st.Atim.Sec),
		atimeNsec: int64(st.Atim.Nsec),
		mtimeSec:  int64(st.Mtim.Sec),
		mtimeNsec: int64(st.Mtim.Nsec),
		ctimeSec:  int64(st.Ctim.Sec),
		ctimeNsec: int64(st.Ctim.Nsec),
		blksize:   int64(st.Blksize),
		blocks:    int64(st.Blocks),
		rdev:      int64(st.Rdev),
	}
}
