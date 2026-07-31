# W0 stdlib gate: runs a curated subset of CPython's own Lib/test modules
# through unittest and exits non-zero if any selected test fails. It locks in
# the text-I/O and path surface unagi already passes and guards it against
# regression as the runtime and the vendored stdlib move.
#
# The vendored modules are the verbatim CPython test files (see unagi-stdlib
# PROVENANCE.toml [scope].reinclude). A handful of individual tests exercise
# capabilities unagi does not have yet; they are excluded by id below and
# tracked as follow-ups, so the gate reflects "everything else must keep
# working" rather than "every upstream test passes".
import sys
import unittest

from test import test_genericpath
from test import test_posixpath

# Known gaps, excluded so the rest of the suite gates cleanly. Each maps to a
# tracked follow-up:
#   - test_import spawns a child interpreter via script_helper; an AOT binary
#     has no interpreter subprocess to spawn.
#   - test_fast_paths_in_use needs the posix._path_splitroot_ex accelerator.
#   - test_realpath_invalid_paths expects realpath to raise UnicodeEncodeError
#     on surrogate paths, which the pure fallback does not.
KNOWN_GAPS = {
    "test.test_posixpath.PosixCommonTest.test_import",
    "test.test_posixpath.PosixPathTest.test_fast_paths_in_use",
    "test.test_posixpath.PosixPathTest.test_realpath_invalid_paths",
}


def flatten(suite):
    for item in suite:
        if isinstance(item, unittest.TestSuite):
            yield from flatten(item)
        else:
            yield item


def build_suite():
    loader = unittest.TestLoader()
    suite = unittest.TestSuite()
    selected = 0
    for module in (test_genericpath, test_posixpath):
        for case in flatten(loader.loadTestsFromModule(module)):
            if case.id() in KNOWN_GAPS:
                continue
            suite.addTest(case)
            selected += 1
    return suite, selected


suite, selected = build_suite()
print("W0 gate: running", selected, "curated tests")
result = unittest.TextTestRunner(verbosity=1).run(suite)
print("W0 gate:", "OK" if result.wasSuccessful() else "FAILED")
sys.exit(0 if result.wasSuccessful() else 1)
