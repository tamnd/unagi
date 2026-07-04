# Augmented assignment on a global that was never bound reads first, so it
# raises NameError before any write, and the traceback names the def frame.
def bump():
    global tally
    tally += 1

bump()
