from codes import *

# The dynamically injected names resolve at module scope after the star import.
TABLE = {ALPHA: 'a', BETA: 'b', GAMMA: 'c'}
KEPT = STATIC


def total():
    return ALPHA + BETA + GAMMA
