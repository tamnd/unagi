# A dict display can splice another mapping with **, evaluated left to right so
# a later key wins over an earlier one, the same order CPython's BUILD_MAP and
# DICT_UPDATE give. Unlike ** in a call, a duplicate key is not an error and a
# key may be any hashable. The spliced value must be a mapping, meaning a
# keys() method whose results index it; anything else raises TypeError.

a = {"x": 1, "y": 2}
b = {"y": 20, "z": 3}

print({**a})
print({**a, **b})
print({**a, "y": 9, **b, "w": 4})
print({0: "zero", **{1: "one"}, 2: "two"})
print({**{}})


class Mapping:
    def keys(self):
        return ["p", "q"]

    def __getitem__(self, k):
        return k * 2


print({**Mapping(), "r": "s"})

try:
    {**5}
except TypeError as e:
    print(e)

try:
    {**[1, 2]}
except TypeError as e:
    print(e)
