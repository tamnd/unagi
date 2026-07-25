package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// _scproxy is the macOS accelerator urllib.request imports on darwin to read the
// system proxy configuration: `from _scproxy import _get_proxy_settings,
// _get_proxies`. Without it `import urllib.request` fails at that top-level
// import on darwin.
//
// The real module calls into the SystemConfiguration framework
// (SCDynamicStoreCopyProxies) through Objective-C, which a pure-Go runtime with
// no cgo cannot reach. So this reports no system-level proxy, the same result
// CPython gives on a machine with nothing configured in the SystemConfiguration
// store. Environment proxies are unaffected: urllib.request's getproxies() is
// `getproxies_environment() or getproxies_macosx_sysconf()` and proxy_bypass
// consults the environment first, so HTTP_PROXY and friends still work; only a
// proxy set solely in System Settings goes unseen.

func init() {
	moduleTable["_scproxy"] = &moduleEntry{builtin: true, exec: initScproxy}
}

func initScproxy(m *objects.Module) error {
	set := func(name string, v objects.Object) error {
		return objects.StoreAttr(m, name, v)
	}
	if err := set("_get_proxies", objects.NewFunc("_get_proxies", 0, scproxyGetProxies)); err != nil {
		return err
	}
	return set("_get_proxy_settings", objects.NewFunc("_get_proxy_settings", 0, scproxyGetProxySettings))
}

// scproxyGetProxies returns the scheme -> proxy-URL mapping, empty because no
// SystemConfiguration store is read. getproxies_macosx_sysconf returns this
// dict verbatim.
func scproxyGetProxies(args []objects.Object) (objects.Object, error) {
	return objects.NewDict(nil, nil)
}

// scproxyGetProxySettings returns the global proxy-bypass settings in the shape
// proxy_bypass_macosx_sysconf expects: exclude_simple off and no exception
// patterns, so nothing is bypassed on our account and the environment decides.
func scproxyGetProxySettings(args []objects.Object) (objects.Object, error) {
	return objects.NewDict(
		[]objects.Object{objects.NewStr("exclude_simple"), objects.NewStr("exceptions")},
		[]objects.Object{objects.False, objects.NewList(nil)},
	)
}
