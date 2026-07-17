package ir

import "github.com/tamnd/unagi/pkg/frontend"

// This file builds the GeneratorResolver a consumer's for-loop drives a module
// generator against. Like the tracked-global and shape resolvers next to it, the
// partitioner and the build both lower consumers that may drive a generator, and
// both must hand the bridge the same set of drivable generators so a consumer
// proven static during partitioning lowers the same way when the build emits it.
// Sharing the construction here keeps the two in step.
//
// Unlike a global or a shape, a generator's drive-site signature needs no partition
// decision to exist: a guard-free generator lowers to a state machine on its own,
// with no callee, global, or shape resolver, so the whole-module table is derivable
// before any function is scored. That is what breaks the apparent chicken-and-egg
// between "a consumer is static only if it can drive the generator" and "the
// generator is emitted only if a static consumer drives it": the generator's
// drivability is a property of the generator alone.

// GeneratorResolverFor builds the drive-site resolver over a module's top-level
// generators. goName maps a generator's Python name to the Go struct type its
// handle is constructed as: the build passes the mangled static name it emits the
// struct under, while the partitioner passes a name it never emits, since it lowers
// a consumer only to measure its cost. A generator the bridge or the signature
// reader refuses, or one goName maps to the empty string, is omitted, so only a
// generator a consumer can actually drive statically resolves. A module with no
// drivable generator returns a nil resolver, which drives nothing and lowers every
// for over a call exactly as the resolver-free bridge did.
func GeneratorResolverFor(m *frontend.Module, goName func(name string) string) GeneratorResolver {
	sigs := map[string]StaticGenerator{}
	for _, s := range m.Body {
		fn, ok := s.(*frontend.FuncDef)
		if !ok || !IsGenerator(fn) {
			continue
		}
		name := goName(fn.Name)
		if name == "" {
			continue
		}
		gen, err := LowerGenerator(fn)
		if err != nil {
			continue
		}
		sig, ok := GeneratorSignatureOf(gen, fn, name)
		if !ok {
			continue
		}
		sigs[fn.Name] = sig
	}
	if len(sigs) == 0 {
		return nil
	}
	return func(name string) (StaticGenerator, bool) {
		sig, ok := sigs[name]
		return sig, ok
	}
}
