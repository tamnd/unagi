package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestScproxyShim checks the _scproxy accelerator reports no system proxy: an
// empty proxy map and bypass settings that exclude nothing, the shape
// urllib.request reads on darwin.
func TestScproxyShim(t *testing.T) {
	m, err := ImportModule("_scproxy")
	if err != nil {
		t.Fatalf("import _scproxy: %v", err)
	}

	getProxies, err := objects.LoadAttr(m, "_get_proxies")
	if err != nil {
		t.Fatalf("_get_proxies attr: %v", err)
	}
	proxies, err := objects.Call(getProxies, nil)
	if err != nil {
		t.Fatalf("_get_proxies(): %v", err)
	}
	if n, err := objects.Len(proxies); err != nil || n != 0 {
		t.Errorf("_get_proxies() len = %d (err %v), want 0", n, err)
	}

	getSettings, err := objects.LoadAttr(m, "_get_proxy_settings")
	if err != nil {
		t.Fatalf("_get_proxy_settings attr: %v", err)
	}
	settings, err := objects.Call(getSettings, nil)
	if err != nil {
		t.Fatalf("_get_proxy_settings(): %v", err)
	}
	excl, err := objects.GetItem(settings, objects.NewStr("exclude_simple"))
	if err != nil {
		t.Fatalf("settings['exclude_simple']: %v", err)
	}
	if b, _ := objects.AsBool(excl); b {
		t.Errorf("exclude_simple = true, want false")
	}
	exc, err := objects.GetItem(settings, objects.NewStr("exceptions"))
	if err != nil {
		t.Fatalf("settings['exceptions']: %v", err)
	}
	if n, err := objects.Len(exc); err != nil || n != 0 {
		t.Errorf("exceptions len = %d (err %v), want 0", n, err)
	}
}
