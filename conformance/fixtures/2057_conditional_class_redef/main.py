# A module-level class defined more than once under mutually-exclusive
# branches, the shape subprocess leans on when it picks one Popen helper class
# per platform. Every definition is conditional, so at most one binds at
# runtime; the lowering gives each its own methods while both assign the one
# shared module variable.

FLAG = True


class Base:
    kind = "base"

    def which(self):
        return "base"


# The first shape carries only class variables, the _del_safe pattern where two
# branches bind a like-named class that differs only in its attributes.
if FLAG:

    class Tag:
        name = "on"
        level = 1

else:

    class Tag:
        name = "off"
        level = 0


print(Tag.name, Tag.level)
print(Tag().name)


# The second shape gives each branch its own methods, so the two like-named
# classes must emit distinct method functions even though they share the key.
if FLAG:

    class Handler(Base):
        mode = "primary"

        def which(self):
            return "primary:" + self.mode

        def tag(self, x):
            return self.mode + "/" + str(x)

else:

    class Handler(Base):
        mode = "fallback"

        def which(self):
            return "fallback:" + self.mode


h = Handler()
print(Handler.mode, h.mode)
print(h.which())
print(h.tag(7))
print(issubclass(Handler, Base), isinstance(h, Base))


# A class only defined inside a branch is a NameError before that branch runs.
try:
    Late()
except NameError as e:
    print("NameError", "Late" in str(e))

if FLAG:

    class Late:
        def hello(self):
            return "late"


print(Late().hello())
