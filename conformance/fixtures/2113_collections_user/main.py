from collections import UserDict, UserList, UserString


class DefaultingDict(UserDict):
    def __missing__(self, key):
        return "?"


d = UserDict({"a": 1})
d["b"] = 2
print(len(d), d["a"], "b" in d, d.get("z", 0))
print(sorted((d | {"x": 10}).items()))
print(sorted(({"y": 9} | d).items()))
d |= {"z": 3}
print(sorted(d.items()))
print(DefaultingDict()["missing"])
print(UserDict.fromkeys(["p", "q"], 0))

l = UserList([1, 2, 3])
print(l + [4], [0] + l, l * 2, 2 * l)
l += [5]
l.append(6)
l.insert(0, 9)
print(l, len(l), l[1:3], l.pop())
print(l == UserList([9, 1, 2, 3, 5]), l < UserList([99]), l.index(2), l.count(1))

s = UserString("Hello")
print(s + " world", "x" + s, s * 2, 2 * s)
print(s.lower(), s.upper(), s.replace("l", "L"))
print(s == "Hello", "ell" in s, s[1:3], len(s))
print("%s!" % s, int(UserString("42")) + 1)
print(type(d).__name__, type(l).__name__, type(s).__name__)
