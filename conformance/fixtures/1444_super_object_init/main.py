# super().__init__() cooperates down to the object root: a subclass reaches its
# base initializer, and once the chain runs out of user classes the object
# default runs as a no-op. A lone class calling super().__init__() lands on that
# default directly, and passing it extra arguments is the object.__init__ error.
class A:
    def __init__(self, x):
        self.x = x


class B(A):
    def __init__(self, x, y):
        super().__init__(x)
        self.y = y


b = B(1, 2)
print(b.x, b.y)


class Solo:
    def __init__(self):
        super().__init__()
        self.z = 9


print(Solo().z)


class Bad:
    def __init__(self):
        super().__init__(1, 2)


try:
    Bad()
except TypeError as e:
    print(e)
