# sysconfig imports: it reads sys._framework on darwin, which used to be absent.
import sysconfig
print("sysconfig" in dir(sysconfig) or True, callable(sysconfig.get_paths))

# site imports without noise: it dedupes set(sys.modules.values()) (needs modules
# hashable), reads sys.copyright, and swallows a missing optional module because
# the ModuleNotFoundError carries the name it checks.
import site
print(hasattr(site, "ENABLE_USER_SITE"))

# ModuleNotFoundError sets .name, the attribute site.execsitecustomize inspects.
try:
    import definitely_not_a_real_module_zzz
except ModuleNotFoundError as e:
    print(e.name, isinstance(e, ImportError))
