// Package sre is the bytecode matcher behind the pure-Python re package.
//
// re parses and compiles a pattern into the SRE bytecode in re._parser and
// re._compiler, then hands the finished bytecode to _sre.compile, which builds
// the compiled pattern (see pkg/objects/sre.go). This package walks that
// bytecode. It is a faithful port of CPython's Modules/_sre engine and depends
// on nothing but the standard library, so the object-model glue that turns a
// run into a Match object lives elsewhere.
package sre

// Opcode values for the SRE bytecode. The ordering and numeric values match the
// OPCODES list re._constants builds, after the two parser-only entries at the
// tail are stripped.
const (
	OpFailure uint32 = iota
	OpSuccess
	OpAny
	OpAnyAll
	OpAssert
	OpAssertNot
	OpAt
	OpBranch
	OpCategory
	OpCharset
	OpBigcharset
	OpGroupref
	OpGrouprefExists
	OpIn
	OpInfo
	OpJump
	OpLiteral
	OpMark
	OpMaxUntil
	OpMinUntil
	OpNotLiteral
	OpNegate
	OpRange
	OpRepeat
	OpRepeatOne
	OpSubpattern
	OpMinRepeatOne
	OpAtomicGroup
	OpPossessiveRepeat
	OpPossessiveRepeatOne
	OpGrouprefIgnore
	OpInIgnore
	OpLiteralIgnore
	OpNotLiteralIgnore
	OpGrouprefLocIgnore
	OpInLocIgnore
	OpLiteralLocIgnore
	OpNotLiteralLocIgnore
	OpGrouprefUniIgnore
	OpInUniIgnore
	OpLiteralUniIgnore
	OpNotLiteralUniIgnore
	OpRangeUniIgnore
)

// Position codes used by OpAt for anchors and word-boundary checks.
const (
	AtBeginning uint32 = iota
	AtBeginningLine
	AtBeginningString
	AtBoundary
	AtNonBoundary
	AtEnd
	AtEndLine
	AtEndString
	AtLocBoundary
	AtLocNonBoundary
	AtUniBoundary
	AtUniNonBoundary
)

// Character-category codes used by OpCategory for \d, \D, \s, \S, \w, \W, the
// linebreak class, and their locale and Unicode variants.
const (
	CategoryDigit uint32 = iota
	CategoryNotDigit
	CategorySpace
	CategoryNotSpace
	CategoryWord
	CategoryNotWord
	CategoryLinebreak
	CategoryNotLinebreak
	CategoryLocWord
	CategoryLocNotWord
	CategoryUniDigit
	CategoryUniNotDigit
	CategoryUniSpace
	CategoryUniNotSpace
	CategoryUniWord
	CategoryUniNotWord
	CategoryUniLinebreak
	CategoryUniNotLinebreak
)

// INFO-block flags appearing in the operand of OpInfo, which re._compiler emits
// at the head of a program to carry the prefix and charset search hints.
const (
	SreInfoPrefix  uint32 = 1
	SreInfoLiteral uint32 = 2
	SreInfoCharset uint32 = 4
)

// Engine identifiers the Python layer reads. MagicNumber stamps the bytecode
// version re._compiler emits, CodeSize is the byte width of one bytecode word in
// CPython's UCS4 packing, MaxRepeat is the unbounded-count sentinel, and
// MaxGroups caps the group count.
const (
	MagicNumber        = 20230612
	CodeSize           = 4
	MaxRepeat   uint32 = 0xFFFFFFFF
	MaxGroups          = 1073741823
)
