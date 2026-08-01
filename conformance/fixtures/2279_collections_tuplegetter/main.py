from _collections import _tuplegetter


class MyT(tuple):
    first = _tuplegetter(0, "the first field")
    second = _tuplegetter(1, "Alias for field number 1")


print("type", type(MyT.first).__name__)
print("repr", repr(MyT.first))
print("doc", MyT.first.__doc__)
MyT.first.__doc__ = "rewritten"
print("doc2", MyT.first.__doc__)
t = MyT((10, 20))
print("attr", t.first, t.second)
print("get", MyT.second.__get__(t, MyT))
print("class-read is descriptor", type(MyT.first).__name__)
