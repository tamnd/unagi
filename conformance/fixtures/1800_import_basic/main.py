import util
import util as u

print(util is u)
print(util.bump(), util.bump(), u.bump())
print(util.count)
print(util.greet("bob"), util.greet("ann", punct="?"))
p = util.Point(3, 4)
print(p.norm())
print(util.Point(1, 2).norm())


def late():
    import util as inner
    return inner


print(late() is util)
print(__name__, util.__name__)
