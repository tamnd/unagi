# functools.update_wrapper makes a wrapper function look like the one it wraps.
# It copies the module, name, qualname, annotations, and doc across, merges the
# wrapped __dict__ into the wrapper's so shared keys take the wrapped value, and
# records the original on wrapper.__wrapped__. A name the wrapped object lacks is
# skipped. functools.wraps is the decorator form, a partial that feeds each
# decorated function through update_wrapper as the wrapper.
import functools


def wrapped(a, b):
    "add two things"
    return a + b


wrapped.shared = "from_wrapped"
wrapped.only_wrapped = 1


def wrapper(*args, **kwargs):
    return None


wrapper.shared = "from_wrapper"
wrapper.only_wrapper = 2

result = functools.update_wrapper(wrapper, wrapped)
print(result is wrapper)
print(wrapper.__name__)
print(wrapper.__qualname__)
print(wrapper.__doc__)
print(wrapper.__module__)
print(wrapper.__wrapped__ is wrapped)
print(sorted(wrapper.__dict__.keys()))
print(wrapper.shared)
print(wrapper.only_wrapped, wrapper.only_wrapper)


@functools.wraps(wrapped)
def decorated(*args, **kwargs):
    return None


print(decorated.__name__)
print(decorated.__doc__)
print(decorated.__wrapped__ is wrapped)
print(decorated.shared)


# A wrapped function with no docstring copies None across.
def plain(x):
    return x


@functools.wraps(plain)
def also(*args, **kwargs):
    "own doc replaced"
    return None


print(also.__name__)
print(repr(also.__doc__))
