print("beta start")
import alpha

try:
    print(alpha.aval)
except AttributeError as e:
    print("attr:", e)

try:
    from alpha import ping
except ImportError as e:
    print("from:", e)

bval = 2
print("beta done")
