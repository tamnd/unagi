# A function defined in this imported module reads a name injected into builtins
# at runtime. It resolves through the imported-module builtins fallback the same
# way the main module resolves an injected name, which is what lets gettext code
# in an imported module see the _ that gettext.install writes into builtins.
def translate():
    return _("hello")


def read_undefined():
    return never_injected_helper_name
