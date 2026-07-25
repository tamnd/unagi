import importlib
import types
import sys


# importlib.import_module drives the pure import machinery, which loops
# sys.meta_path finders. unagi resolves an `import name` statement in its own Go
# importer, so meta_path would be empty and import_module would raise. The native
# finder bridges the two, so import_module returns the same module the statement
# would.
for name in ["json", "os", "re", "typing"]:
    m = importlib.import_module(name)
    print(name, m.__name__)


# A dotted submodule resolves the same way.
eu = importlib.import_module("email.utils")
print(eu.__name__)


# import_module hands back the very object already in sys.modules, so a second
# call and the statement form agree on identity.
import json

print(importlib.import_module("json") is json)
print("json" in sys.modules)


# The finder is a normal meta_path entry, so a function-valued attribute on a
# SimpleNamespace is callable through method syntax, the shape the machinery uses
# when it calls finder.find_spec(name).
ns = types.SimpleNamespace(f=lambda x: x + 1)
print(ns.f(41))
