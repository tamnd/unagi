package ir

import (
	"testing"

	"github.com/tamnd/unagi/pkg/emit"
)

// identityName maps a generator name to itself, the placeholder the partitioner
// passes since it never emits the struct.
func identityName(name string) string { return name }

// TestGeneratorResolverForResolvesModuleGenerator proves the whole-module builder
// finds a top-level generator, reads its drive-site signature, and reports it under
// its Python name, so a consumer's for loop can construct and drive it.
func TestGeneratorResolverForResolvesModuleGenerator(t *testing.T) {
	m := parseModule(t, "def gen(n: int):\n    for i in range(n):\n        yield i\n")
	res := GeneratorResolverFor(m, identityName)
	if res == nil {
		t.Fatal("a module with a drivable generator should build a resolver")
	}
	sig, ok := res("gen")
	if !ok {
		t.Fatal("the resolver should report the module generator")
	}
	if sig.GoName != "gen" {
		t.Fatalf("the handle type should be the name the builder mapped, got %q", sig.GoName)
	}
	if sig.Elem.Scalar != emit.SInt {
		t.Fatalf("the counting generator yields int, got %+v", sig.Elem)
	}
	if len(sig.Params) != 1 || !sig.Params[0].Saved {
		t.Fatalf("n is read across the suspension, so it should be a saved param: %+v", sig.Params)
	}
}

// TestGeneratorResolverForNamesWithMapper proves the builder constructs each
// signature under the Go name the caller's mapper returns, the seam the build uses
// to name the handle by the mangled static struct while the partitioner names it by
// the bare Python name.
func TestGeneratorResolverForNamesWithMapper(t *testing.T) {
	m := parseModule(t, "def gen(n: int):\n    for i in range(n):\n        yield i\n")
	res := GeneratorResolverFor(m, func(name string) string { return "static_test_" + name })
	sig, ok := res("gen")
	if !ok {
		t.Fatal("the resolver should report the module generator")
	}
	if sig.GoName != "static_test_gen" {
		t.Fatalf("the handle type should be the mapped name, got %q", sig.GoName)
	}
}

// TestGeneratorResolverForSkipsMappedEmptyName proves a generator the mapper maps to
// the empty string is omitted, the way the build drops a generator that was not
// decided static and so carries no emitted struct name.
func TestGeneratorResolverForSkipsMappedEmptyName(t *testing.T) {
	m := parseModule(t, "def gen(n: int):\n    for i in range(n):\n        yield i\n")
	res := GeneratorResolverFor(m, func(string) string { return "" })
	if res != nil {
		t.Fatalf("a generator mapped to no name should leave the resolver empty, got %v", res)
	}
}

// TestGeneratorResolverForSkipsUnlowerableGenerator proves a generator the bridge
// refuses is not in the drivable set, so a consumer finds nothing to drive and stays
// boxed. Here the unannotated parameter blocks the signature read.
func TestGeneratorResolverForSkipsUnlowerableGenerator(t *testing.T) {
	m := parseModule(t, "def gen(a):\n    yield a\n")
	if res := GeneratorResolverFor(m, identityName); res != nil {
		t.Fatalf("an unlowerable generator should leave the resolver empty, got %v", res)
	}
}

// TestGeneratorResolverForNoGeneratorsIsNil proves a module with no generator builds
// a nil resolver, which drives nothing and lowers every for over a call exactly as
// the resolver-free bridge did.
func TestGeneratorResolverForNoGeneratorsIsNil(t *testing.T) {
	m := parseModule(t, "def f(a: int) -> int:\n    return a + 1\n")
	if res := GeneratorResolverFor(m, identityName); res != nil {
		t.Fatalf("a module with no generator should build no resolver, got %v", res)
	}
}
