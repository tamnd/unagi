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
# A set of individual tests exercise codec-error machinery unagi does not carry
# yet; they are excluded by method name below and tracked as follow-ups, so the
# gate reflects "the codec tables and everything else must keep working" rather
# than "every upstream test passes". The common thread across the excluded
# codecencodings tests is the structured codec-error object: a runtime-raised
# UnicodeEncodeError/UnicodeDecodeError does not carry start/end/object/reason
# yet, and the xmlcharrefreplace error handler is not registered, so the error
# callback and custom-replace paths cannot run.
#
# The other section 7 codec suites are not gated here yet and are tracked
# separately:
#   - test_codecs has a broad surface gap set (codecs.escape_decode/encode and
#     readbuffer_encode are absent, the charmap encoding is not registered, the
#     text/binary transform codecs raise, and the standard error handlers
#     backslashreplace/surrogateescape are missing).
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
# across every encoding it covers). The codecencodings entries all trace to the
# missing structured codec-error object and the unregistered xmlcharrefreplace
# handler:
#   - test_callback_* and test_customreplace_encode read .end/.object off the
#     raised UnicodeEncodeError, which unagi does not populate yet.
#   - test_incrementalencoder, test_incrementalencoder_error_callback,
#     test_streamreader and test_streamwriter drive the incremental/stream
#     error path through the same object.
#   - test_incrementalencoder_del_segfault expects reading .errors on a
#     half-initialized encoder to raise AttributeError; unagi does not.
#   - test_xmlcharrefreplace needs the xmlcharrefreplace error handler.
#   - test_errorhandle, test_chunkcoding and test_incrementaldecoder hit the
#     same error-object/handler gaps on a subset of the encodings.
# The test_multibytecodec entries track the getstate/setstate codec state
# protocol and its validation, the custom error-callback handlers, and the
# MultibyteStreamReader argument check, none of which unagi carries yet.
KNOWN_GAP_METHODS = {
    "test_callback_None_index",
    "test_callback_backward_index",
    "test_callback_forward_index",
    "test_callback_index_outofbound",
    "test_callback_long_index",
    "test_callback_returns_bytes",
    "test_callback_wrong_objects",
    "test_customreplace_encode",
    "test_incrementalencoder",
    "test_incrementalencoder_del_segfault",
    "test_incrementalencoder_error_callback",
    "test_streamreader",
    "test_streamwriter",
    "test_xmlcharrefreplace",
    "test_errorhandle",
    "test_chunkcoding",
    "test_incrementaldecoder",
    "test_errorcallback_custom_ignore",
    "test_errorcallback_longindex",
    "test_getstate_returns_expected_value",
    "test_init_segfault",
    "test_issue5640",
    "test_setstate_validates_input",
    "test_setstate_validates_input_bytes",
    "test_setstate_validates_input_size",
    "test_state_methods",
    "test_state_methods_with_buffer_state",
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
