package partition

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tamnd/unagi/pkg/frontend"
)

// drive parses src and runs the phase driver over it, failing the test on a
// parse error so a fixture typo is loud.
func drive(t *testing.T, src string) []Decision {
	t.Helper()
	m, err := frontend.Parse([]byte(src), "app.py")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return Drive("app", m)
}

// byName indexes a decision set by unit name for lookups.
func byName(ds []Decision) map[string]Decision {
	out := make(map[string]Decision, len(ds))
	for _, d := range ds {
		out[d.Unit.Name] = d
	}
	return out
}

func TestDriveEnumeratesEveryBody(t *testing.T) {
	src := `
x = 1

def top(a):
    return a + 1

class C:
    def method(self):
        return 2

pairs = [i * i for i in range(10)]
f = lambda z: z + 1
`
	ds := drive(t, src)
	got := byName(ds)
	want := []string{
		ModuleUnitName,
		"<module>.top",
		"<module>.C",
		"<module>.C.method",
		"<module>.<listcomp>",
		"<module>.<lambda>",
	}
	for _, name := range want {
		if _, ok := got[name]; !ok {
			t.Errorf("missing unit %q; got %v", name, names(ds))
		}
	}
	if len(ds) != len(want) {
		t.Errorf("unit count = %d, want %d: %v", len(ds), len(want), names(ds))
	}
}

func TestDriveBoxesUnprovenUnits(t *testing.T) {
	// A function the bridge cannot lower (an unannotated parameter) carries no
	// static proof, so it boxes on the cost model. The module top level boxes too,
	// as module bodies always do at M4.
	ds := drive(t, "def f(a):\n    return a + 1\n")
	for _, d := range ds {
		if d.State.IsStatic() {
			t.Errorf("unit %q went static without a proof; it should box", d.Unit.Name)
		}
	}
}

func TestDriveBoxesAsyncDef(t *testing.T) {
	// An async def builds a coroutine, which has no static form at M4: the bridge
	// refuses it, so the unit carries no proof and stays boxed even though its body
	// is a plain scalar return the sync twin would prove static. This is the tier
	// the 08:42 checklist pins for the async fixtures.
	ds := byName(drive(t, "async def inc(x: int) -> int:\n    return x + 1\n"))
	d, ok := ds["<module>.inc"]
	if !ok {
		t.Fatalf("missing unit <module>.inc; got %v", names(drive(t, "async def inc(x: int) -> int:\n    return x + 1\n")))
	}
	if d.State.IsStatic() {
		t.Fatalf("an async def has no static form at M4 and must box, got %v", d.State)
	}
}

func TestDriveProvesScalarFunctionStatic(t *testing.T) {
	// A proven scalar function with total float arithmetic clears the cost model
	// and lands static, the first tier the partitioner proves through the bridge.
	ds := byName(drive(t, "def f(a: float, b: float, c: float) -> float:\n    return a * b + c\n"))
	d, ok := ds["<module>.f"]
	if !ok {
		t.Fatal("missing unit <module>.f")
	}
	if d.State != StaticProven {
		t.Fatalf("a total float function should prove static, got %v with %+v", d.State, d.Reasons)
	}
	if d.Score.Static >= d.Score.Boxed {
		t.Errorf("static score %d should beat boxed %d", d.Score.Static, d.Score.Boxed)
	}
	// The module unit that runs the def statement stays boxed; only the function
	// body is proven.
	if ds[ModuleUnitName].State.IsStatic() {
		t.Errorf("the module body should stay boxed, got %v", ds[ModuleUnitName].State)
	}
}

func TestDriveProvesCallerOfStaticCalleeStatic(t *testing.T) {
	// A caller of another static function is boxed on the first decision pass, since
	// the bridge refuses the call until the callee is known static. The call-graph
	// fixpoint feeds the proven callee back in, so a later pass lowers the call and
	// the caller proves static too. Both are total float, so neither carries a guard.
	src := "def scale(a: float, b: float) -> float:\n" +
		"    return a * b\n\n" +
		"def outer(a: float, b: float) -> float:\n" +
		"    return scale(a, b) + a\n"
	ds := byName(drive(t, src))
	if d := ds["<module>.scale"]; d.State != StaticProven {
		t.Fatalf("the callee should prove static, got %v with %+v", d.State, d.Reasons)
	}
	d, ok := ds["<module>.outer"]
	if !ok {
		t.Fatal("missing unit <module>.outer")
	}
	if d.State != StaticProven {
		t.Fatalf("a caller of a static callee should prove static once the fixpoint resolves the call, got %v with %+v", d.State, d.Reasons)
	}
	if len(d.Deopts) != 0 {
		t.Errorf("a total float caller carries no deopt site, got %d", len(d.Deopts))
	}
}

func TestDriveBoxesCallerOfBoxedCallee(t *testing.T) {
	// The fixpoint only resolves a callee that actually proves static. A callee the
	// bridge cannot lower (an unannotated parameter) never enters the resolver, so
	// its caller keeps refusing the call and stays boxed, never guessing a shape.
	src := "def helper(a):\n" +
		"    return a + 1\n\n" +
		"def outer(a: float, b: float) -> float:\n" +
		"    return helper(a) + b\n"
	ds := byName(drive(t, src))
	if d := ds["<module>.outer"]; d.State.IsStatic() {
		t.Errorf("a caller of a boxed callee should stay boxed, got %v", d.State)
	}
}

func TestDriveBoxesGuardHeavyIntFunction(t *testing.T) {
	// A short int function is all guarded adds, so its overflow guards outweigh
	// the unboxed work and the guard budget demotes it. The tier is honest that a
	// tiny int computation is not worth leaving the boxed twin for.
	ds := byName(drive(t, "def f(a: int, b: int) -> int:\n    return a + b\n"))
	d := ds["<module>.f"]
	if d.State.IsStatic() {
		t.Errorf("a single guarded int add should not go static, got %v", d.State)
	}
	if d.State != BoxedByCost {
		t.Errorf("it should box on the cost model, got %v", d.State)
	}
}

func TestDriveStaticFunctionCarriesDeoptSites(t *testing.T) {
	// A proven static function that still carries one overflow guard lands static
	// and hands its boxed twin a deopt plan: one statement-boundary resume point,
	// a rebox transfer per live scalar, and a VerifyPlan-clean set of sites. The
	// body is float-heavy enough to clear the guard budget (fifteen unboxed ops
	// against a single int guard), with the lone int add `(m + n)` supplying the
	// guard the return statement deopts on.
	src := "def f(a: float, b: float, m: int, n: int) -> float:\n" +
		"    return a * b + a * b + a * b + a * b + a * b + a * b + a * b + (m + n)\n"
	d, ok := byName(drive(t, src))["<module>.f"]
	if !ok {
		t.Fatal("missing unit <module>.f")
	}
	if d.State != StaticProven {
		t.Fatalf("function should prove static, got %v with %+v", d.State, d.Reasons)
	}
	if len(d.Deopts) == 0 {
		t.Fatal("a static function with a guard should carry a deopt site")
	}
	if v := VerifyPlan(d.Deopts); len(v) != 0 {
		t.Fatalf("deopt plan should verify clean, got %+v", v)
	}
	site := d.Deopts[0]
	if site.Resume.Kind != ResumeStatement {
		t.Errorf("resume point should sit at a statement boundary, got %v", site.Resume.Kind)
	}
	// The four parameters are all live entering the return statement, and each
	// gets a rebox transfer so the boxed twin rebuilds its frame.
	if site.LiveCount() != 4 {
		t.Errorf("live count = %d, want 4 (a, b, m, n)", site.LiveCount())
	}
	if len(site.Transfers) != site.LiveCount() {
		t.Errorf("transfer count = %d, want one per live var (%d)", len(site.Transfers), site.LiveCount())
	}
	for _, tr := range site.Transfers {
		if tr.Kind != MatRebox {
			t.Errorf("live scalar %q should rebox, got %v", tr.Native, tr.Kind)
		}
		if tr.Type == nil {
			t.Errorf("transfer for %q carries no lattice type", tr.Native)
		}
	}
}

func TestDriveProvesSubscriptFunctionStaticWithBoundsDeopt(t *testing.T) {
	// A function that builds a scalar list local and reads it by index proves static
	// and carries the bounds guard as a deopt site: an out-of-range or negative index
	// hands the boxed twin the read at the statement boundary, so the boxed side owns
	// the negative-index wraparound and the IndexError. The body is float-heavy enough
	// to clear the guard budget, with the lone bounds guard on the return being the
	// site the twin resumes at.
	src := "def probe(a: float, b: float, i: int) -> float:\n" +
		"    xs = [1.5, 2.5, 3.5]\n" +
		"    return a * b + a * b + a * b + a * b + a * b + a * b + a * b + xs[i]\n"
	d, ok := byName(drive(t, src))["<module>.probe"]
	if !ok {
		t.Fatal("missing unit <module>.probe")
	}
	if d.State != StaticProven {
		t.Fatalf("subscript function should prove static, got %v with %+v", d.State, d.Reasons)
	}
	if len(d.Deopts) != 1 {
		t.Fatalf("the read should carry exactly one bounds deopt site, got %d", len(d.Deopts))
	}
	if v := VerifyPlan(d.Deopts); len(v) != 0 {
		t.Fatalf("deopt plan should verify clean, got %+v", v)
	}
	site := d.Deopts[0]
	if site.Resume.Kind != ResumeStatement {
		t.Errorf("resume point should sit at a statement boundary, got %v", site.Resume.Kind)
	}
	// a, b, and i are the live scalars entering the return; the list local xs is an
	// aggregate the boxed twin rebuilds from the top, so it is not a rebox transfer.
	if site.LiveCount() != 3 {
		t.Errorf("live count = %d, want 3 (a, b, i)", site.LiveCount())
	}
	for _, tr := range site.Transfers {
		if tr.Kind != MatRebox {
			t.Errorf("live scalar %q should rebox, got %v", tr.Native, tr.Kind)
		}
	}
}

// driveWith parses src and runs the phase driver under an explicit tier mode.
func driveWith(t *testing.T, src string, mode Mode) []Decision {
	t.Helper()
	m, err := frontend.Parse([]byte(src), "app.py")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return DriveWith("app", m, mode)
}

func TestDriveForceStaticEmitsCostLosingFunction(t *testing.T) {
	// A single guarded int add auto-boxes on the guard budget (see
	// TestDriveBoxesGuardHeavyIntFunction). Forced static emits its static form
	// anyway so the harness can diff the guard-heavy static build against the
	// boxed build. The function lowers, so the tier lever reaches it.
	src := "def f(a: int, b: int) -> int:\n    return a + b\n"
	auto := byName(driveWith(t, src, ModeAuto))["<module>.f"]
	if auto.State.IsStatic() {
		t.Fatalf("precondition: this function should auto-box, got %v", auto.State)
	}
	forced := byName(driveWith(t, src, ModeForceStatic))["<module>.f"]
	if forced.State != StaticProven {
		t.Fatalf("forced static should emit the function static, got %v with %+v", forced.State, forced.Reasons)
	}
}

func TestDriveForceStaticSkipsUnlowerableAndModuleBodies(t *testing.T) {
	// Forced static only reaches units the bridge lowered. A function with an
	// unannotated parameter never lowers, so it falls back to auto and stays
	// boxed rather than being forced into a static form that does not exist, and
	// the module body (never a lowered unit) stays boxed too.
	src := "def f(a):\n    return a + 1\n"
	ds := byName(driveWith(t, src, ModeForceStatic))
	if d := ds["<module>.f"]; d.State.IsStatic() {
		t.Errorf("an unlowerable function must not be forced static, got %v", d.State)
	}
	if d := ds[ModuleUnitName]; d.State.IsStatic() {
		t.Errorf("the module body must not be forced static, got %v", d.State)
	}
}

func TestDriveForceBoxedDemotesProvenStatic(t *testing.T) {
	// A total float function proves static under auto (see
	// TestDriveProvesScalarFunctionStatic). Forced boxed overrides that so the
	// same program runs through the boxed tier for the differential, and the
	// report names the forced-boxed rule.
	src := "def f(a: float, b: float, c: float) -> float:\n    return a * b + c\n"
	auto := byName(driveWith(t, src, ModeAuto))["<module>.f"]
	if auto.State != StaticProven {
		t.Fatalf("precondition: this function should auto-prove static, got %v", auto.State)
	}
	forced := byName(driveWith(t, src, ModeForceBoxed))["<module>.f"]
	if forced.State != BoxedByCost {
		t.Fatalf("forced boxed should demote the function, got %v", forced.State)
	}
	if len(forced.Reasons) != 1 || forced.Reasons[0].Rule != RuleTierForcedBoxed {
		t.Fatalf("want a tier-forced-boxed reason, got %+v", forced.Reasons)
	}
}

func TestDriveForceStaticStillBoxesEvalUnit(t *testing.T) {
	// eval is a hard census disqualifier, so even forced static keeps the unit
	// boxed: there is no sound static form to force for a genuinely dynamic body.
	src := "def f(s):\n    return eval(s)\n"
	d := byName(driveWith(t, src, ModeForceStatic))["<module>.f"]
	if d.State != BoxedByCensus {
		t.Fatalf("forced static must not override a census disqualifier, got %v", d.State)
	}
}

func TestDriveRecordsEvalAsCensusDisqualifier(t *testing.T) {
	ds := byName(drive(t, "def f(s):\n    return eval(s)\n"))
	d, ok := ds["<module>.f"]
	if !ok {
		t.Fatal("missing unit <module>.f")
	}
	if d.State != BoxedByCensus {
		t.Fatalf("eval(s) should box its unit by census, got %v", d.State)
	}
	if len(d.Reasons) != 1 || d.Reasons[0].Rule != RuleEvalDynamicSource {
		t.Fatalf("want a single eval-dynamic-source reason, got %+v", d.Reasons)
	}
}

func TestDriveAllowsConstantEvalSource(t *testing.T) {
	// eval of a constant string is not a census disqualifier: the source is
	// visible to the compiler. The unit still boxes, but on the cost model, not
	// on a hard census fact.
	ds := byName(drive(t, "def f():\n    return eval('1 + 1')\n"))
	d := ds["<module>.f"]
	if d.State == BoxedByCensus {
		t.Fatalf("constant eval source must not record a census disqualifier: %+v", d.Reasons)
	}
	if d.State != BoxedByCost {
		t.Fatalf("unit should box on the cost model, got %v", d.State)
	}
}

func TestDriveRecordsLocalsAndFrameWalkers(t *testing.T) {
	cases := []struct {
		name string
		body string
		rule string
	}{
		{"locals", "    return locals()", RuleLocalsCall},
		{"getframe", "    return sys._getframe()", RuleFrameWalkerDirect},
		{"currentframe", "    return inspect.currentframe()", RuleFrameWalkerDirect},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ds := byName(drive(t, "def f():\n"+tc.body+"\n"))
			d := ds["<module>.f"]
			if d.State != BoxedByCensus {
				t.Fatalf("%s should box by census, got %v", tc.name, d.State)
			}
			if len(d.Reasons) != 1 || d.Reasons[0].Rule != tc.rule {
				t.Fatalf("want reason %s, got %+v", tc.rule, d.Reasons)
			}
		})
	}
}

func TestDriveSeesDisqualifierNestedInArguments(t *testing.T) {
	// eval buried in an argument list still boxes the unit: the walk descends
	// every subexpression, so the census cannot be dodged by nesting.
	ds := byName(drive(t, "def f(s):\n    return print(1 + len(eval(s)))\n"))
	d := ds["<module>.f"]
	if d.State != BoxedByCensus {
		t.Fatalf("nested eval should still box by census, got %v with %+v", d.State, d.Reasons)
	}
}

func TestDriveAttributesDisqualifierToInnermostUnit(t *testing.T) {
	// eval inside a nested function belongs to that function's unit, not the
	// enclosing one. The outer function trips no hard rule and boxes on cost.
	src := `
def outer():
    def inner(s):
        return eval(s)
    return inner
`
	ds := byName(drive(t, src))
	if got := ds["<module>.outer.inner"].State; got != BoxedByCensus {
		t.Errorf("inner should box by census, got %v", got)
	}
	if got := ds["<module>.outer"].State; got != BoxedByCost {
		t.Errorf("outer trips no hard rule, should box on cost, got %v", got)
	}
}

func TestDriveIsDeterministic(t *testing.T) {
	src := "def f(a):\n    return [x for x in a if x]\n"
	m, err := frontend.Parse([]byte(src), "app.py")
	if err != nil {
		t.Fatal(err)
	}
	a := DecisionHash(Drive("app", m))
	b := DecisionHash(Drive("app", m))
	if a != b {
		t.Fatalf("driver output is not deterministic:\n%s\n%s", a, b)
	}
}

// TestDriveHandlesTheWholeCorpus drives every fixture's entry source through
// the walk. It is a robustness check: the walk must cover the full grammar the
// corpus exercises (match, try, with, async, f-strings, patterns) without
// panicking, and every unit it enumerates must decide. It asserts nothing about
// tiers, only that the front door survives real programs.
func TestDriveHandlesTheWholeCorpus(t *testing.T) {
	root := filepath.Join("..", "..", "conformance", "fixtures")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Skipf("corpus not found: %v", err)
	}
	drove := 0
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		path := filepath.Join(root, ent.Name(), "main.py")
		src, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		m, err := frontend.Parse(src, path)
		if err != nil {
			// A fixture that deliberately probes a SyntaxError does not parse;
			// it is not a driver concern.
			continue
		}
		ds := Drive(ent.Name(), m)
		if len(ds) == 0 {
			t.Errorf("%s drove to an empty decision set", ent.Name())
		}
		// The module top level is always a unit.
		if ds[0].Unit.Name != ModuleUnitName && !hasModuleUnit(ds) {
			t.Errorf("%s produced no module unit: %v", ent.Name(), names(ds))
		}
		drove++
	}
	if drove == 0 {
		t.Fatal("drove no fixtures; corpus path is wrong")
	}
	t.Logf("drove %d fixtures", drove)
}

func hasModuleUnit(ds []Decision) bool {
	for _, d := range ds {
		if d.Unit.Name == ModuleUnitName {
			return true
		}
	}
	return false
}

// names lists the unit names in a decision set for failure messages.
func names(ds []Decision) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = d.Unit.Name
	}
	return out
}
