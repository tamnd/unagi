class D(dict):
    def __missing__(self, k):
        v = k.upper()
        self[k] = v
        return v

d = D(a="1")
print(d.__getitem__("a"))
print(d.__getitem__("z"))        # __missing__ -> "Z", stored
print(d.__len__())
print(d.__contains__("a"), d.__contains__("q"))
d.__setitem__("b", "2")
print(sorted(d.items()))
d.__delitem__("b")
print(sorted(d.items()))
g = D().__getitem__              # bound-method factory, the _Quoter shape
print(g("x"))                    # "X"

from urllib.parse import quote, unquote
print(quote("a b/c?x=1"))
print(unquote("a%20b%2Fc"))
