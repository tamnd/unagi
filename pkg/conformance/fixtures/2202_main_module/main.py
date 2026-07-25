import __main__
import rlcompleter

print(type(__main__).__name__)
print(__main__.__name__)
print(isinstance(__main__.__dict__, dict))

MARKERVALUE = 42
g = globals()
print(__main__.__dict__ is g)
print(__main__.__dict__["MARKERVALUE"])

c = rlcompleter.Completer()
print(c.complete("MARKERVAL", 0))
print(c.complete("zzznope", 0))
