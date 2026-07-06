import alpha

print(alpha.aval, alpha.beta.bval)
print(alpha.ping())
from alpha import ping
print(ping())
import beta
print(beta.alpha is alpha)
