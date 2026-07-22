//go:build darwin

package runtime

import (
	"syscall"

	"github.com/tamnd/unagi/pkg/objects"
)

// posixStatExtraNames are the stat_result fields darwin adds past the common
// set, in repr order. They match the fields darwin CPython's stat_result exposes.
var posixStatExtraNames = []string{"st_flags", "st_gen", "st_birthtime", "st_birthtime_ns"}

// statNormalize reads a darwin syscall.Stat_t into the host-independent carrier.
// darwin names the time fields Atimespec/Mtimespec/Ctimespec/Birthtimespec.
func statNormalize(st *syscall.Stat_t) statNormal {
	birthSec, birthNsec := int64(st.Birthtimespec.Sec), int64(st.Birthtimespec.Nsec)
	return statNormal{
		mode:      int64(st.Mode),
		ino:       int64(st.Ino),
		dev:       int64(st.Dev),
		nlink:     int64(st.Nlink),
		uid:       int64(st.Uid),
		gid:       int64(st.Gid),
		size:      int64(st.Size),
		atimeSec:  int64(st.Atimespec.Sec),
		atimeNsec: int64(st.Atimespec.Nsec),
		mtimeSec:  int64(st.Mtimespec.Sec),
		mtimeNsec: int64(st.Mtimespec.Nsec),
		ctimeSec:  int64(st.Ctimespec.Sec),
		ctimeNsec: int64(st.Ctimespec.Nsec),
		blksize:   int64(st.Blksize),
		blocks:    int64(st.Blocks),
		rdev:      int64(st.Rdev),
		extras: []objects.Object{
			objects.NewInt(int64(st.Flags)),
			objects.NewInt(int64(st.Gen)),
			objects.NewFloat(float64(birthSec) + float64(birthNsec)/1e9),
			objects.NewInt(birthSec*1_000_000_000 + birthNsec),
		},
	}
}
