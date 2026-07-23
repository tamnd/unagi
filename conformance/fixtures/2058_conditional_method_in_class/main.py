# A method defined inside a conditional block in a class body, the shape
# subprocess.Popen uses when it defines its platform-specific helpers under
# if _mswindows. The guard is a module-level flag, so the branch that runs binds
# its methods into the class namespace and the other set never binds; a method
# outside the guard binds unconditionally, and the ordinals stay aligned across
# the mix.

MSWINDOWS = False


class Popen:
    tag = "popen"

    def common(self):
        return "common:" + self.tag

    if MSWINDOWS:

        def handles(self):
            return "win-handles"

        def wait(self):
            return "win-wait"

    else:

        def handles(self):
            return "posix-handles"

        def wait(self):
            return "posix-wait"

    def close(self):
        return "close:" + self.handles()


p = Popen()
print(p.common())
print(p.handles())
print(p.wait())
print(p.close())
print(hasattr(Popen, "handles"), hasattr(Popen, "common"))


# The other branch of the same class shape, to show the guard actually selects
# which definition binds rather than always taking the first.
WINDOWS2 = True


class Sock:
    def kind(self):
        return "sock"

    if WINDOWS2:

        def send(self, n):
            return "win-send:" + str(n)

    else:

        def send(self, n):
            return "posix-send:" + str(n)


print(Sock().send(3))


# A decorated method under a guard still runs its decorator, so a staticmethod
# defined conditionally is callable off the class.
FEATURE = True


class Tool:
    if FEATURE:

        @staticmethod
        def make():
            return "made"

    def run(self):
        return self.make() + "/run"


print(Tool.make())
print(Tool().run())


# A conditional method that calls a zero-argument super() resolves the class
# through the same cell an unconditional method reads, so the guarded override
# still reaches its base.
class Animal:
    def speak(self):
        return "animal"


PROVIDE = True


class Dog(Animal):
    if PROVIDE:

        def speak(self):
            return "dog+" + super().speak()


print(Dog().speak())
