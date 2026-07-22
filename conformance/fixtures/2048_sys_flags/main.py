# sys.flags is the structseq of interpreter startup flags. A compiled program
# runs none of the command-line switches these report, so they carry CPython's
# default startup values. _py_warnings reads sys.flags.context_aware_warnings at
# import, and warnings reads sys.warnoptions, so both have to be present for the
# warnings and traceback import chain.
import sys

print(sys.flags.context_aware_warnings)
print(sys.flags.thread_inherit_context)
print(sys.flags.gil)
print(sys.flags.hash_randomization)
print(sys.flags.int_max_str_digits)
print(sys.flags.dev_mode)
print(sys.flags.safe_path)
print(sys.flags.optimize)

# The first n_sequence_fields entries are the visible tuple.
print(sys.flags[0], sys.flags[11])
print(len(sys.flags))
print(sys.flags.n_sequence_fields, sys.flags.n_fields, sys.flags.n_unnamed_fields)
print(tuple(sys.flags))

print(sys.warnoptions)
print(type(sys.warnoptions).__name__)
