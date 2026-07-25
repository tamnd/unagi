import abc


class A(abc.ABC):
    @abc.abstractmethod
    def f(self):
        ...


try:
    A()
except TypeError as e:
    print(e)


class B(abc.ABC):
    @abc.abstractmethod
    def f(self):
        ...

    @abc.abstractmethod
    def g(self):
        ...


try:
    B()
except TypeError as e:
    print(e)


# A class built with type(...) inherits the abstract methods and is blocked too.
X = type("X", (A,), {})
try:
    X()
except TypeError as e:
    print(e)


# Ending a user __new__ chain at object.__new__ hits the same gate.
class N(abc.ABC):
    @abc.abstractmethod
    def f(self):
        ...

    def __new__(cls):
        return super().__new__(cls)


try:
    N()
except TypeError as e:
    print(e)


# A concrete subclass that implements every abstract method instantiates fine.
class C(A):
    def f(self):
        return 3


print(C().f())
