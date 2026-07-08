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

func TestDriveBoxesEveryUnitAtM4(t *testing.T) {
	// With no proof feed, every enumerated unit boxes. The typed tier proves
	// nothing static through this path yet, and the driver reports that honestly.
	ds := drive(t, "def f(a):\n    return a + 1\n")
	for _, d := range ds {
		if d.State.IsStatic() {
			t.Errorf("unit %q went static at M4; nothing should be proven static yet", d.Unit.Name)
		}
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
