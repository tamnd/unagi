# S1 stdlib gate: runs a curated subset of CPython's own Lib/test suites for
# the pure-Python modules S1 lights up (graphlib, heapq, bisect, textwrap)
# through unittest and exits non-zero if any selected test fails. It locks in
# the behavior these modules already hold over the W0 native floor and guards
# it against regression as the runtime and the vendored stdlib move.
#
# The vendored modules are the verbatim CPython test files (see unagi-stdlib
# PROVENANCE.toml [scope].reinclude). A handful of individual tests exercise
# capabilities unagi does not have yet; they are excluded by method name below
# and tracked as follow-ups, so the gate reflects "everything else must keep
# working" rather than "every upstream test passes".
#
# The other section 7 suites are not gated here yet and are tracked separately:
#   - test_pprint has a broad set of repr gaps (dataclass repr, mappingproxy,
#     OrderedDict repr, set ordering) that make it too noisy to gate cleanly.
#   - test_collections has a wide gap set (60/101 today).
#   - test_weakref crashes the process through the weakref items generator.
#   - test_contextlib crashes on a SyntaxError attribute read.
#   - test_copy and test_types do not AOT-compile yet; test_functools and
#     test_enum fail at import time.
# Each is a tracked follow-up, folded in as the underlying gap closes.
import sys
import unittest

from test import test_graphlib
from test import test_heapq
from test import test_bisect
from test import test_textwrap

# Known gaps, excluded by method name so the concrete TestCase class does not
# matter (heapq and bisect each run the same suite twice, once over the C
# accelerator and once over the pure fallback). Each maps to a tracked
# follow-up:
#   - test_static_order_does_not_change_with_the_hash_seed spawns a child
#     interpreter via script_helper; an AOT binary has none to spawn.
#   - test_prepare_cycleerror_each_time expects prepare() to re-raise the same
#     CycleError on each call; unagi lets the second call raise uncaught.
#   - test_comparison_operator_modifying_heap mutates the heap from inside a
#     comparison and expects the partial order to survive; unagi surfaces the
#     bare TypeError from the mixed comparison instead.
#   - test_c_functions and test_py_functions read heapq function __module__,
#     which unagi's function objects do not carry yet.
#   - test_lookups_with_key_function passes key=str.casefold as an unbound
#     method; unagi does not expose str.casefold as an unbound descriptor.
#   - test_non_breaking_space and test_narrow_non_breaking_space expect the
#     wrapper to treat U+00A0 / U+202F as non-breaking; unagi breaks on them.
KNOWN_GAP_METHODS = {
    "test_static_order_does_not_change_with_the_hash_seed",
    "test_prepare_cycleerror_each_time",
    "test_comparison_operator_modifying_heap",
    "test_c_functions",
    "test_py_functions",
    "test_lookups_with_key_function",
    "test_non_breaking_space",
    "test_narrow_non_breaking_space",
}


def excluded(case):
    return case.id().rsplit(".", 1)[-1] in KNOWN_GAP_METHODS


def flatten(suite):
    for item in suite:
        if isinstance(item, unittest.TestSuite):
            yield from flatten(item)
        else:
            yield item


# The suites are loaded straight from their TestCase subclasses rather than
# through loadTestsFromModule, which would fire each module's load_tests hook
# and pull in doctest suites over the live module. Those doctest suites do not
# collect under unagi yet, and they are not what this gate is guarding; the
# class-based tests are.
def build_suite():
    loader = unittest.TestLoader()
    suite = unittest.TestSuite()
    selected = 0
    for module in (test_graphlib, test_heapq, test_bisect, test_textwrap):
        for value in vars(module).values():
            if isinstance(value, type) and issubclass(value, unittest.TestCase):
                for case in flatten(loader.loadTestsFromTestCase(value)):
                    if excluded(case):
                        continue
                    suite.addTest(case)
                    selected += 1
    return suite, selected


suite, selected = build_suite()
print("S1 gate: running", selected, "curated tests")
result = unittest.TextTestRunner(verbosity=1).run(suite)
print("S1 gate:", "OK" if result.wasSuccessful() else "FAILED")
sys.exit(0 if result.wasSuccessful() else 1)
