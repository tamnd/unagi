import sys
import warnings

# Attributes present with the value CPython carries, or a golden-stable shape
# where the value is host specific.
print(isinstance(sys._framework, str))
print(isinstance(sys.abiflags, str))
print("Python Software Foundation" in sys.copyright, sys.copyright.count("All Rights Reserved") == 4)
print(isinstance(sys.dont_write_bytecode, bool))
print(sys.pycache_prefix is None)
print(type(sys.path_hooks).__name__, type(sys.meta_path).__name__, type(sys.path_importer_cache).__name__)

# Introspection and interning.
print(sys.intern("hello") == "hello", type(sys.intern("hi")).__name__)
print(sys.getdefaultencoding())
print(sys.is_finalizing())
print(sys.getallocatedblocks() >= 0, type(sys.getallocatedblocks()).__name__)
with warnings.catch_warnings():
    warnings.simplefilter("ignore")
    print(sys._clear_type_cache() is None)

# Trace and profile hooks store and return, starting unset.
def hook(*a):
    return hook
print(sys.gettrace())
sys.settrace(hook)
print(sys.gettrace() is hook)
sys.settrace(None)
print(sys.gettrace())
print(sys.getprofile())
sys.setprofile(hook)
print(sys.getprofile() is hook)
sys.setprofile(None)

# call_tracing runs the function and returns None.
seen = []
r = sys.call_tracing(lambda a, b: seen.append((a, b)), (1, 2))
print(r, seen)
