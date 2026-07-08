# super read as a value, not just called in place. copyreg registers super in
# its dispatch table (pickle(super, pickle_super)), so the name has to resolve
# to the super type object, repr as a class, compare by identity, and work as a
# dict key. The two-argument call form has to keep working when it is reached
# through a stored reference instead of the bare super() syntax.

print(super)
print(super is super)
print(repr(super))

d = {super: "registered"}
print(d[super])
print(super in d)


class A:
    def who(self):
        return "A"


class B(A):
    def who(self):
        return "B"


class C(B):
    def who(self):
        # zero-argument super() still uses its own lowering.
        return super().who()

    def grand(self):
        # a stored super reference, called with the explicit two-argument form.
        s = super
        return s(B, self).who()


c = C()
print(c.who())
print(c.grand())

alias = super
print(alias(C, c).who())

try:
    super()
except RuntimeError as e:
    print("runtime:", e)
