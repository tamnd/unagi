import sysconfig

# get_config_vars returns a dict, and platlibdir is the fixed POSIX default,
# both host-invariant and identical to CPython.
print(type(sysconfig.get_config_vars()) is dict)
print(sysconfig.get_config_var('platlibdir'))
print(sysconfig.get_config_var('py_version_short'))

# sysconfig imports the platform-specific _sysconfigdata build module, so pydoc
# and zoneinfo, which reach get_config_vars/get_config_var through it, import.
import zoneinfo
import pydoc
print(zoneinfo.ZoneInfo.__name__)
print(hasattr(pydoc, 'help'))
print(callable(sysconfig.get_paths))
