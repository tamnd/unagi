# test.support is the CPython test-helper package the conformance harness leans
# on. It is now vendored into the embedded stdlib (the test/support subtree
# re-included past the Lib/test exclude), and with the three module-level
# runtime gaps closed (float.__getformat__, sys._jit, strftime with no tuple)
# its module body runs to completion. This pins that the package imports and
# exposes the surface the harness reads, without touching the helper submodules
# that still need unicodedata NFD.
import test
import test.support as support

print("package", test.__name__, support.__name__)

# The decorators and predicates the harness keys on are present and callable.
print("requires_IEEE_754", callable(support.requires_IEEE_754))
print("cpython_only", callable(support.cpython_only))
print("check_impl_detail", callable(support.check_impl_detail))
print("gc_collect", callable(support.gc_collect))
print("requires_resource", callable(support.requires_resource))

# TestFailed is the exception type the harness raises; it is a real class.
print("TestFailed", isinstance(support.TestFailed, type),
      issubclass(support.TestFailed, Exception))

# A couple of the plain data constants the body computes at import.
print("verbose is int", isinstance(support.verbose, int))
print("has LOOPBACK_TIMEOUT", isinstance(support.LOOPBACK_TIMEOUT, float))
print("SHORT_TIMEOUT float", isinstance(support.SHORT_TIMEOUT, float))
print("max_memuse int", isinstance(support.max_memuse, int))
