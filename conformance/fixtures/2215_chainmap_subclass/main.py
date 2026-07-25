# collections.ChainMap is subclassable. unittest's case.py derives
# _OrderedChainMap(collections.ChainMap) to reorder the maps in __iter__, so
# `import unittest` failed at class-creation time with "bases must be types"
# until a ChainMap subclass could take the chainMapObject layout. This exercises
# the subclass mechanics and then the unittest surface the fix unblocks.

import collections


class OrderedChainMap(collections.ChainMap):
    # The exact override case.py uses: walk the maps front to back, yield each
    # key once, in first-seen order.
    def __iter__(self):
        seen = set()
        for mapping in self.maps:
            for k in mapping:
                if k not in seen:
                    seen.add(k)
                    yield k


c = OrderedChainMap({"a": 1}, {"b": 2, "a": 3})

# A subclass instance is a ChainMap, and it is not a plain ChainMap type.
print(isinstance(c, collections.ChainMap))
print(type(c).__name__)

# Mapping reads resolve through the payload: a lookup walks the maps, the first
# hit wins.
print(c["a"], c["b"])
print(c.get("b"), c.get("missing", -1))
print("a" in c, "z" in c)
print(len(c))

# self.maps is the live list the override walks; iteration uses the override.
print(list(c))
print(c.maps == [{"a": 1}, {"b": 2, "a": 3}])

# Writes and deletes go through to the first mapping.
c["z"] = 9
print(c["z"], c.maps[0] == {"a": 1, "z": 9})
del c["z"]
print("z" in c)

# Inherited ChainMap methods act on the payload: new_child pushes a fresh map,
# parents drops the first.
child = c.new_child({"n": 5})
print(child["n"], child["a"])
print(dict(sorted(c.parents.items())) if hasattr(c, "parents") else "no parents",
      list(c.parents.maps) == [{"b": 2, "a": 3}])

# The wall the fix cleared: unittest imports and a TestCase is usable directly.
import unittest


class Direct(unittest.TestCase):
    def runTest(self):
        pass


t = Direct()
t.assertEqual(1 + 1, 2)
t.assertNotEqual(1, 2)
t.assertTrue([1])
t.assertFalse([])
t.assertIn("a", {"a": 1})
t.assertDictEqual({"x": 1, "y": 2}, {"y": 2, "x": 1})
t.assertListEqual([1, 2], [1, 2])
t.assertSetEqual({1, 2}, {2, 1})
try:
    t.assertEqual(1, 2)
    print("no raise")
except AssertionError:
    print("assertion raised on mismatch")

print("done")
