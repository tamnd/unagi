import _typing

print(_typing._idfunc(5))
print(_typing._idfunc("hi"))
lst = [1, 2, 3]
print(_typing._idfunc(lst) is lst)

n = _typing.NoDefault
print(repr(n))
print(str(n))
print(n.__reduce__())
print(n is _typing.NoDefault)
