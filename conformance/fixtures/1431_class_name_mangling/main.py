class Account:
    __count = 0

    def __init__(self, owner, balance):
        self.__owner = owner
        self.__balance = balance
        Account.__count += 1

    def deposit(self, amount):
        self.__balance += amount
        return self.__balance

    def __audit(self):
        return f"{self.__owner}:{self.__balance}"

    def report(self):
        return self.__audit()

    def seen(self):
        return Account.__count


class _Ledger:
    def __init__(self):
        self.__entries = []

    def add(self, tag, value):
        __row = (tag, value)
        self.__entries.append(__row)
        return len(self.__entries)

    def tags(self):
        return [__e[0] for __e in self.__entries]


a = Account("alice", 100)
b = Account("bob", 50)
print(a.deposit(25))
print(a.report())
print(b.report())
print(a.seen())

L = _Ledger()
print(L.add("x", 1))
print(L.add("y", 2))
print(L.tags())
