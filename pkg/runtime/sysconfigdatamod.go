package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// _sysconfigdata_<abiflags>_<platform>_<multiarch> is the module CPython's
// build writes with a build_time_vars dict of every configure/Makefile value,
// which sysconfig reads back through _get_sysconfigdata. The name is computed
// at runtime from sys.abiflags/sys.platform/sys.implementation._multiarch, so
// it varies by platform and cannot be a static table entry; the import machinery
// routes any `_sysconfigdata_`-prefixed name here.
//
// An AOT-compiled unagi program was never produced by a CPython build, so there
// is no genuine build database to report. This synthesizes a small, honest
// subset: sysconfig merges build_time_vars as a base and then overlays the paths
// it computes itself from the install schemes and sys attributes, so an empty or
// minimal base still yields working get_paths/get_config_vars results, which is
// all the importers on the current sweep (pydoc, zoneinfo through
// sysconfig.get_config_var("TZPATH")) need. It is deliberately not a fabricated
// full compiler configuration.

func initSysconfigData(m *objects.Module) error {
	vars, err := objects.NewDict(nil, nil)
	if err != nil {
		return err
	}
	return objects.StoreAttr(m, "build_time_vars", vars)
}
