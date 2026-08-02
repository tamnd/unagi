# S2 stdlib gate: runs CPython's own CJK codec suites (test_codecencodings_cn,
# _hk, _jp, _kr, _tw, _iso2022), test_multibytecodec and test_unicodedata
# through unittest and exits non-zero if any selected test fails. It locks in
# the multibyte codec conversion tables and the incremental/stream codec surface
# S2 lights up plus the pinned unicodedata property surface, and guards them
# against regression as the runtime and the vendored stdlib move.
#
# The vendored modules are the verbatim CPython test files, plus the
# cjkencodings/*.txt reference strings the suites read at runtime (see
# unagi-stdlib PROVENANCE.toml [scope].reinclude). The gate test copies that
# data next to the binary and runs from there, since the suites locate it
# through os.path.dirname(__file__).
#
# Every class-based test in these suites now runs: the structured codec-error
# object, the standard ignore/replace/xmlcharrefreplace/backslashreplace handlers
# and the error callback path (a registered handler's returned replacement and
# newpos steering the codec loop) are all in place, the encoder and decoder
# getstate/setstate state protocols run, deleting an encoder or decoder .errors
# handler raises AttributeError the way the C getset does, the incremental encoder
# and decoder run end to end (including euc_kr's eight-byte Hangul make-up sequence
# across a chunk boundary), the iso-2022 decode replace path runs across every
# encoding, and the native stream reader and writer drive read/readline/readlines
# and write/writelines end to end. So KNOWN_GAP_METHODS is empty and the gate runs
# the full curated suite.
#
# The other section 7 codec suites are not gated here yet and are tracked
# separately:
#   - test_codecs has a broad surface gap set (codecs.escape_decode/encode and
#     readbuffer_encode are absent, the charmap encoding is not registered, the
#     text/binary transform codecs raise, and the namereplace/surrogateescape
#     error handlers are missing).
#   - test_locale reads os.uname() in its darwin setUpClass, which unagi does
#     not expose, so the real-locale base class errors on macOS.
#   - test_gettext depends on gettext.install's injected _ builtin and on
#     writing .mo files to disk under the test directory.
# Each is a tracked follow-up, folded in as the underlying gap closes.
import sys
import unittest

from test import test_codecencodings_cn
from test import test_codecencodings_hk
from test import test_codecencodings_jp
from test import test_codecencodings_kr
from test import test_codecencodings_tw
from test import test_codecencodings_iso2022
from test import test_multibytecodec
from test import test_unicodedata

# No method-name exclusions remain in the codec suites: every class-based test in
# the six codecencodings suites and test_multibytecodec passes, including the
# stream reader/writer surface (test_streamreader, test_streamwriter) and the
# bare MultibyteStreamReader/Writer(None) AttributeError guard (test_init_segfault).
# The set is kept so a future gap can be parked here without reworking the loop.
KNOWN_GAP_METHODS = set()

# test_unicodedata pieces held out of the gate are genuine data or CPython
# implementation gaps, not regressions. Unicode_3_2_0_FunctionsTest answers from
# a true UCD 3.2.0 database unagi does not carry (unicodedata.ucd_3_2_0 is
# modeled over the current UCD), and UnicodeMiscTest.test_ucd_510 is the same
# 3.2.0-data gap. NormalizationTest reads the NormalizationTest.txt data file and
# gates on the cpu/network resources. The remaining three UnicodeMiscTest cases
# are CPython C-implementation details: disallowing instantiation of the C type,
# a subprocess probe of a failed import during compiling, and a C-extension
# unload/reload. Everything else in test_unicodedata, the whole current-UCD
# property and case surface including both the method and function full
# checksums, runs green.
SKIP_CLASSES = {"Unicode_3_2_0_FunctionsTest", "NormalizationTest"}
SKIP_METHODS = {
    ("UnicodeMiscTest", "test_ucd_510"),
    ("UnicodeMiscTest", "test_disallow_instantiation"),
    ("UnicodeMiscTest", "test_failed_import_during_compiling"),
    ("UnicodeMiscTest", "test_unicodedata_unload_reload"),
}


def excluded(case):
    parts = case.id().split(".")
    cls, meth = parts[-2], parts[-1]
    if meth in KNOWN_GAP_METHODS:
        return True
    if cls in SKIP_CLASSES:
        return True
    return (cls, meth) in SKIP_METHODS


def flatten(suite):
    for item in suite:
        if isinstance(item, unittest.TestSuite):
            yield from flatten(item)
        else:
            yield item


# The suites are loaded straight from their TestCase subclasses rather than
# through loadTestsFromModule, which would fire each module's load_tests hook;
# the class-based tests are what this gate guards.
def build_suite():
    loader = unittest.TestLoader()
    suite = unittest.TestSuite()
    selected = 0
    modules = (
        test_codecencodings_cn,
        test_codecencodings_hk,
        test_codecencodings_jp,
        test_codecencodings_kr,
        test_codecencodings_tw,
        test_codecencodings_iso2022,
        test_multibytecodec,
        test_unicodedata,
    )
    for module in modules:
        for value in vars(module).values():
            if isinstance(value, type) and issubclass(value, unittest.TestCase):
                for case in flatten(loader.loadTestsFromTestCase(value)):
                    if excluded(case):
                        continue
                    suite.addTest(case)
                    selected += 1
    return suite, selected


suite, selected = build_suite()
print("S2 gate: running", selected, "curated tests")
result = unittest.TextTestRunner(verbosity=1).run(suite)
print("S2 gate:", "OK" if result.wasSuccessful() else "FAILED")
sys.exit(0 if result.wasSuccessful() else 1)
