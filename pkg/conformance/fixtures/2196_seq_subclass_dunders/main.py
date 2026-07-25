class L(list):
    pass

l = L([10, 20, 30, 40])
print(l.__getitem__(1))          # operator vs attribute agree
print(l.__getitem__(-1))         # negative index
print(l.__getitem__(slice(1, 3)))
print(l.__len__())
print(l.__contains__(20), l.__contains__(99))
l.__setitem__(0, 99)
print(l[0])
l.__delitem__(0)
print(list(l))
g = L([1, 2, 3]).__getitem__     # bound-method factory
print(g(0), g(-1))


class T(tuple):
    pass

t = T((1, 2, 3, 4))
print(t.__getitem__(2))
print(t.__getitem__(-1))
print(t.__getitem__(slice(None, None, 2)))
print(t.__len__())
print(t.__contains__(3), t.__contains__(9))


class M(list):
    def __getitem__(self, i):
        return "override"

print(M([1, 2]).__getitem__(0))  # user override still wins
