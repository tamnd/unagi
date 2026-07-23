# A module-level function defined more than once under mutually-exclusive
# branches, the shape ntpath and inspect lean on when a name is bound to one of
# several platform or feature specific implementations. Every definition is
# conditional, so at most one binds at runtime; the lowering gives each its own
# Go function while both assign the one shared module variable.

FLAG = True

if FLAG:

    def pick(x):
        return "on:" + str(x)

else:

    def pick(x):
        return "off:" + str(x)


print(pick(1))
print(pick("a"))


# Both definitions carry a default parameter, so each needs its own default
# slot; the branch that runs binds the slot the running def reads.
if FLAG:

    def greet(name, sep=": "):
        return "hi" + sep + name

else:

    def greet(name, sep=" - "):
        return "hey" + sep + name


print(greet("sam"))
print(greet("sam", sep="! "))


# A name only defined inside a branch is a NameError before that branch runs.
try:
    later()
except NameError as e:
    print("NameError", "later" in str(e))

if FLAG:

    def later():
        return "defined"


print(later())
