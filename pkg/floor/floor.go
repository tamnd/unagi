// Package floor holds the pure-Python standard library floor: the stdlib
// modules unagi compiles from source as ordinary input rather than backing
// with Go. The sources come from tamnd/unagi-stdlib, an embed.FS of the pinned
// CPython 3.14.6 Lib tree copied verbatim, so a compiled program runs the same
// module body the oracle runs and the two stay byte-identical. They are
// third-party code under the Python Software Foundation License and are not
// edited: fixes go upstream, not here.
//
// The build resolves a floor import against FS when the name does not resolve
// next to the entry file and is not one the runtime provides itself as a Go
// module. A Go accelerator a floor module reaches for (its C counterpart, such
// as _stat behind stat) is a separate built-in module in pkg/runtime; a module
// that only reaches for it behind a guarded import compiles and runs without
// one. Because the tree is embedded in the compiler binary, resolution needs no
// source checkout on disk and works the same from a working tree, `go install`,
// or the module cache.
package floor

import (
	"io/fs"

	unagistdlib "github.com/tamnd/unagi-stdlib"
)

// FS returns the floor source tree, rooted so that a dotted module name maps
// straight onto a path within it: "os" is os.py, "json" is json/__init__.py,
// "xml.etree.ElementTree" is xml/etree/ElementTree.py. It subs into the "Lib"
// prefix the vendored embed carries, which is the same rooting CPython uses
// when it resolves an import against its Lib directory.
func FS() (fs.FS, error) {
	return fs.Sub(unagistdlib.FS(), "Lib")
}
