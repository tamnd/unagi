# S2 stdlib gate: runs CPython's own CJK codec suites (test_codecencodings_cn,
# _hk, _jp, _kr, _tw, _iso2022) and test_multibytecodec through unittest and
# exits non-zero if any selected test fails. It locks in the multibyte codec
# conversion tables and the incremental/stream codec surface S2 lights up and
# guards them against regression as the runtime and the vendored stdlib move.
#
# The vendored modules are the verbatim CPython test files, plus the
# cjkencodings/*.txt reference strings the suites read at runtime (see
# unagi-stdlib PROVENANCE.toml [scope].reinclude). The gate test copies that
# data next to the binary and runs from there, since the suites locate it
# through os.path.dirname(__file__).
#
# A set of individual tests exercise codec machinery unagi does not carry yet;
# they are excluded by method name below and tracked as follow-ups, so the gate
# reflects "the codec tables and everything else must keep working" rather than
# "every upstream test passes". The structured codec-error object, the standard
# ignore/replace/xmlcharrefreplace/backslashreplace handlers, and the error
# callback path (a registered handler's returned replacement and newpos steering
# the codec loop) are all in place, so the callback and custom-replace tests run
# in the gate. The encoder and decoder getstate/setstate state protocols run
# too, and deleting an encoder or decoder .errors handler raises AttributeError
# the way the C getset does. The incremental encoder and decoder now run end to
# end, including euc_kr's eight-byte Hangul make-up sequence across a chunk
# boundary, and the iso-2022 decode replace path now runs across every encoding,
# so the remaining exclusions are the stream reader and writer.
#
# The other section 7 codec suites are not gated here yet and are tracked
# separately:
#   - test_codecs has a broad surface gap set (codecs.escape_decode/encode and
#     readbuffer_encode are absent, the charmap encoding is not registered, the
#     text/binary transform codecs raise, and the namereplace/surrogateescape
#     error handlers are missing).
#   - test_unicodedata needs the full UCD property tables (category, numeric,
#     decimal, bidirectional, east_asian_width, the name and decomposition
#     data, and the function/method checksums) which are only partly present.
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

# Known gaps, excluded by method name so the concrete per-encoding TestCase
# class does not matter (each codecencodings module defines the same method set
# across every encoding it covers). The error-callback path itself now runs (a
# registered handler's returned replacement and newpos steer the codec loop,
# including backward and forward positions, str and bytes replacements, and the
# type/bounds validation), the encoder and decoder getstate/setstate state
# protocols run, deleting an encoder or decoder .errors handler now raises
# AttributeError the way the C getset does, and the incremental encoder and
# decoder (test_incrementalencoder, test_incrementaldecoder, test_chunkcoding)
# run across every encoding, and test_errorhandle (the iso-2022 decode replace
# path, plus every codec's replace/strict codectests tuples) runs too, so the
# remaining exclusions are the stream surface:
#   - test_streamreader and test_streamwriter drive the stream read and write
#     path, which unagi does not carry end to end yet.
# test_init_segfault (MultibyteStreamReader/Writer(None) raising AttributeError)
# is the remaining test_multibytecodec entry, tracked with the stream surface.
KNOWN_GAP_METHODS = {
    "test_streamreader",
    "test_streamwriter",
    "test_init_segfault",
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
