// Package floor holds the pure-Python standard library floor: the stdlib
// modules unagi compiles from source as ordinary input rather than backing
// with Go. The sources under lib are copied verbatim from the pinned CPython
// 3.14.6 tree, so a compiled program runs the same module body the oracle runs
// and the two stay byte-identical. They are third-party code under the Python
// Software Foundation License and are not edited: fixes go upstream, not here.
//
// The build locates these sources through LibSubdir, relative to the unagi
// source tree, and resolves a floor import there when it does not resolve next
// to the entry file. A Go accelerator a floor module reaches for (its C
// counterpart, such as _stat behind stat) is a separate built-in module in
// pkg/runtime; a module that only reaches for it behind a guarded import
// compiles and runs without one.
package floor

// LibSubdir is where the bundled floor sources live, relative to the root of
// the unagi source tree. The build joins it onto the located tree to form the
// floor search root.
const LibSubdir = "pkg/floor/lib"
