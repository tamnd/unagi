# An entry script that imports unittest now reifies the __main__ module, binding
# its top-level TestCase classes onto sys.modules['__main__'] so unittest can
# discover them there — the mechanism unittest.main() relies on when a test file
# is run directly. Ordinary scripts keep the un-reified fast path.
import sys
import unittest


class MathTests(unittest.TestCase):
    def test_add(self):
        self.assertEqual(1 + 1, 2)

    def test_sub(self):
        self.assertEqual(5 - 3, 2)


class StrTests(unittest.TestCase):
    def test_in(self):
        self.assertIn("x", "axb")


main_mod = sys.modules["__main__"]
print("name", main_mod.__name__)
print("classes", sorted(n for n in dir(main_mod) if n.endswith("Tests")))

loader = unittest.TestLoader()
suite = loader.loadTestsFromModule(main_mod)
print("count", suite.countTestCases())

result = unittest.TestResult()
suite.run(result)
print("run", result.testsRun)
print("ok", result.wasSuccessful())
print("failures", len(result.failures), "errors", len(result.errors))
