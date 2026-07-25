# sys.exc_info() and BaseException.with_traceback were the last two walls on the
# unittest run path: testPartExecutor reads exc_info() to record a failure, and
# assertRaises's __exit__ does exc.with_traceback(None). This exercises both
# directly and then a full failing/erroring/assertRaises suite through TestResult.

import sys
import unittest

# Outside any except block exc_info() is the (None, None, None) triple.
print(sys.exc_info())

# Inside an except block it names the handled exception: its type and the value.
# The traceback slot is host-variant (CPython carries a real traceback object,
# unagi models none), so it is deliberately not printed here.
try:
    raise ValueError("boom")
except ValueError:
    t, v, tb = sys.exc_info()
    print(t is ValueError, isinstance(v, ValueError), str(v))
    # sys.exception() is the value half of the same triple.
    print(sys.exception() is v)

# with_traceback returns self and leaves __traceback__ None; both the call form
# and re-raise form work.
e = KeyError("k")
print(e.with_traceback(None) is e, e.__traceback__ is None)
try:
    raise IndexError("i").with_traceback(None)
except IndexError as ie:
    print(str(ie))


class Suite(unittest.TestCase):
    def test_pass(self):
        self.assertEqual(2 + 2, 4)

    def test_fail(self):
        self.assertEqual(1, 2)

    def test_error(self):
        raise RuntimeError("kaboom")

    def test_raises_ok(self):
        with self.assertRaises(ZeroDivisionError):
            1 / 0

    def test_raises_regex(self):
        with self.assertRaisesRegex(KeyError, "ab"):
            raise KeyError("abc")


suite = unittest.TestLoader().loadTestsFromTestCase(Suite)
result = unittest.TestResult()
suite.run(result)
print(result.testsRun, len(result.failures), len(result.errors), result.wasSuccessful())

# The recorded failure and error carry the expected exception text.
fail_msgs = "".join(msg for _, msg in result.failures)
err_msgs = "".join(msg for _, msg in result.errors)
print("AssertionError" in fail_msgs, "RuntimeError: kaboom" in err_msgs)

print("done")
