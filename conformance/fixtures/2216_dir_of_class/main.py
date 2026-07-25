# dir() of a type object raised "dir() of a 'type' object is not supported yet".
# unittest's loader walks dir(TestCaseClass) to collect the test methods, so the
# whole run stack was blocked past class creation even once the class built. dir
# of a class gathers the object base set plus every name across the class's own
# MRO, which is exactly what CPython reports.

import unittest


class Base:
    def helper(self):
        pass

    shared = 1


class Sub(Base):
    x = 2

    def foo(self):
        pass

    def test_a(self):
        pass

    def test_b(self):
        pass


# The full dir of a class is the sorted object base set plus the class and base
# namespaces; every name here is fixed, so the list is host-invariant.
print(dir(Sub))

# An inherited name shows up, a subclass name shows up, and a dunder from the
# object base shows up.
print("helper" in dir(Sub), "foo" in dir(Sub), "shared" in dir(Sub))
print("__init__" in dir(Sub), "__dict__" in dir(Sub))

# The metaclass-only names CPython leaves out of dir(cls) stay out.
print("__mro__" in dir(Sub), "__bases__" in dir(Sub), "__call__" in dir(Sub))

# dir of an empty class is the object base set alone.
class Empty:
    pass


print(dir(Empty))

# The wall this clears: unittest's loader collects test methods off a TestCase by
# walking dir(cls), and a passing suite runs to completion through TestResult.
class MyTests(unittest.TestCase):
    def test_one(self):
        self.assertEqual(1 + 1, 2)

    def test_two(self):
        self.assertIn("a", "abc")
        self.assertTrue([1])

    def not_a_test(self):
        raise RuntimeError("never collected")


loader = unittest.TestLoader()
print(loader.getTestCaseNames(MyTests))

suite = loader.loadTestsFromTestCase(MyTests)
result = unittest.TestResult()
suite.run(result)
print(result.testsRun, result.wasSuccessful())
print(len(result.failures), len(result.errors))

print("done")
