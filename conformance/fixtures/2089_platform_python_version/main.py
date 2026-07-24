import platform

# platform derives these from sys.version and sys.version_info, so they read the
# Python version rather than anything about the host. The build, compiler and
# uname values platform also exposes are host-specific and are left out.
print("implementation", platform.python_implementation())
print("version", platform.python_version())
print("version_tuple", platform.python_version_tuple())

# branch and revision come from sys._git, which a compiled program does not
# carry, so they are the empty strings CPython reports for a build without it.
print("branch", repr(platform.python_branch()))
print("revision", repr(platform.python_revision()))
