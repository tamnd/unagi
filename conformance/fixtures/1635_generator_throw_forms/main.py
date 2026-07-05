# generator.throw injects an exception at the current yield. The single-argument
# form takes an exception instance thrown as itself or a class instantiated with
# no arguments; a value that does not derive from BaseException is the throw
# TypeError. The exception propagates out of the generator when the body does not
# catch it.


def g():
    yield 1
    yield 2
    yield 3


# An instance is thrown as itself and propagates with its args.
gen = g()
print(next(gen))
try:
    gen.throw(ValueError("boom"))
except ValueError as e:
    print("inst", e.args)


# A bare class is instantiated with no arguments.
gen = g()
print(next(gen))
try:
    gen.throw(KeyError)
except KeyError as e:
    print("cls", e.args)


# A user exception subclass throws and propagates the same way.
class AppError(Exception):
    pass


gen = g()
print(next(gen))
try:
    gen.throw(AppError("bad"))
except AppError as e:
    print("user", e.args)


# A non-exception value is the throw-specific TypeError.
gen = g()
print(next(gen))
try:
    gen.throw(42)
except TypeError as e:
    print("nonexc", e)


# Throwing into an unstarted generator still runs the injection and propagates.
gen2 = g()
try:
    gen2.throw(RuntimeError("early"))
except RuntimeError as e:
    print("early", e.args)
