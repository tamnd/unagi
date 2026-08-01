# A builtin container type exposes its protocol special methods as unbound
# method-wrappers off the type object, the way CPython's wrapper_descriptors do,
# so collections/__init__.py can bind dict_setitem = dict.__setitem__ at class
# body time and call it as dict_setitem(self, key, value). The same flip needs a
# tuple-subclass to %-format through its element tuple, dict.__new__ to allocate
# a subclass instance, and a function's __defaults__ to be writable so the
# eval'd namedtuple __new__ can take its per-field defaults. Exercise each one.

# Unbound container dunders read off the type resolve to method-wrappers that run
# the same operator the bound receiver.__dunder__ does.
d = {}
dict.__setitem__(d, "a", 1)
dict.__setitem__(d, "b", 2)
print(dict.__getitem__(d, "a"), dict.__len__(d), dict.__contains__(d, "b"))
dict.__delitem__(d, "a")
print("a" in d, list(d.items()))

lst = [10, 20, 30]
list.__setitem__(lst, 1, 99)
print(lst, list.__getitem__(lst, 2), list.__len__(lst))

# The read-only sequence surface answers size, membership and subscript but not
# assignment; a set answers only size and membership.
t = (5, 6, 7)
print(tuple.__getitem__(t, 0), tuple.__len__(t), tuple.__contains__(t, 6))
print(str.__len__("hello"), str.__contains__("hello", "ell"))
print(set.__len__({1, 2, 3}), set.__contains__({1, 2, 3}, 2))

# The descriptor guards its receiver type the way CPython's does.
try:
    dict.__setitem__([], "k", "v")
except TypeError as e:
    print("guarded:", "requires a 'dict' object" in str(e))

# dict.__new__/list.__new__ allocate an instance of the (sub)class, which a
# subclass __new__ chains to before its own __init__ runs.
class MyDict(dict):
    pass

md = dict.__new__(MyDict)
print(type(md).__name__, isinstance(md, dict), len(md))

# A tuple subclass %-formats through its element tuple, so "%s/%s" % self unpacks
# the fields the way namedtuple's generated __repr__ relies on.
class Pair(tuple):
    def __repr__(self):
        return "Pair(%s, %s)" % self

print(repr(Pair((1, 2))))
print("%d-%d-%d" % Pair((3, 4, 5)))

# A function's __defaults__ is writable: assigning a tuple makes the trailing
# positional parameters optional, honored at call time, and reads back as that
# tuple. This is how namedtuple applies per-field defaults to its eval'd __new__.
def make(a, b, c):
    return (a, b, c)

make.__defaults__ = (100, 200)
caller = make  # force a dynamic call so the compile-time binding is not used
print(caller(1), caller(1, 2), caller(1, 2, 3))
print(make.__defaults__)

# Clearing with None drops every positional default again.
make.__defaults__ = None
print(make.__defaults__)

print("done")
