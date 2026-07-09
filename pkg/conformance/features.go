package conformance

// Feature is one landed M4 static lowering case from the checklist docs 01
// through 09. The differential band (doc 10 items 9 and 10) requires that every
// such case map to at least one fixture, so the corpus proves the lowering is
// byte-identical to CPython rather than only unit-tested. A Feature carries the
// tag a fixture writes in its fixture.toml, the doc it comes from, and a short
// description of the case.
type Feature struct {
	Tag  string // the string a fixture lists under [tags]
	Doc  string // the checklist doc the case lives in, e.g. "02"
	Desc string // the lowering case, one line
}

// Features is the registry of landed static lowering cases the corpus must
// cover. It is deliberately the set of cases that emit a static form at M4: the
// integer and float arithmetic, the comparisons and connectives, the control
// flow, and the resolved static call and recursion. The M5-blocked cases
// (subscripts, attributes, containers, rebindable globals, generator produce
// and consume, in-body boxed excursions) are not listed, because there is no
// static lowering to hold a fixture to yet; they join the registry with their
// forms. The coverage test asserts both directions: every feature here has a
// fixture, and every tag a fixture writes is a feature here.
var Features = []Feature{
	// doc 02, integer arithmetic
	{"int-add", "02", "int + int lowers to a guarded native add"},
	{"int-sub", "02", "int - int lowers to a guarded native subtract"},
	{"int-mul", "02", "int * int lowers to a guarded native multiply"},
	{"int-floordiv", "02", "int // int lowers to the flooring divide"},
	{"int-mod", "02", "int % int lowers to the flooring modulo"},
	{"int-pow", "02", "int ** int lowers to the guarded power"},
	{"int-bitand", "02", "int & int lowers to a native and"},
	{"int-bitor", "02", "int | int lowers to a native or"},
	{"int-bitxor", "02", "int ^ int lowers to a native xor"},
	{"int-lshift", "02", "int << int lowers to the guarded left shift"},
	{"int-rshift", "02", "int >> int lowers to the native right shift"},
	{"int-const-fold", "02", "a constant int expression folds to a literal"},
	{"int-identity", "02", "an int identity op collapses to the operand"},

	// doc 03, float arithmetic and coercion
	{"float-arith", "03", "float op float lowers to the native float op"},
	{"float-int-coerce", "03", "mixed float and int coerces to float"},
	{"float-repr", "03", "a float prints CPython's shortest round-trip repr"},

	// doc 05, booleans, comparisons, connectives
	{"compare-int", "05", "int comparisons lower to native relational ops"},
	{"compare-chain", "05", "a chained comparison lowers without reevaluation"},
	{"bool-and", "05", "and short-circuits to the operand value"},
	{"bool-or", "05", "or short-circuits to the operand value"},
	{"bool-not", "05", "not lowers to a native negation"},
	{"truthiness", "05", "a scalar in a condition lowers to its truth test"},

	// doc 06, statements and control flow
	{"if-elif-else", "06", "an if/elif/else chain lowers to native branches"},
	{"while", "06", "a while loop lowers to a native loop"},
	{"break-continue", "06", "break and continue lower to loop control"},
	{"for-range", "06", "for over range lowers to a counted native loop"},
	{"augassign", "06", "an augmented assignment lowers in place"},

	// doc 07, calls and functions
	{"static-call", "07", "a resolved static callee lowers to a direct call"},
	{"recursion", "07", "a self-recursive static function lowers directly"},
}

// featureTags returns the registered tags as a set.
func featureTags() map[string]Feature {
	m := make(map[string]Feature, len(Features))
	for _, f := range Features {
		m[f.Tag] = f
	}
	return m
}
