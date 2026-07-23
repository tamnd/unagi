package runtime

import (
	"fmt"
	"strings"

	"github.com/tamnd/unagi/pkg/objects"
)

// time.struct_time is the structseq the pure-Python datetime builds in
// _build_struct_time and hands to strftime. Its nine sequence fields are the
// calendar breakdown; tm_zone and tm_gmtoff are named-only and read None when
// the value comes from a plain nine-tuple, as datetime's always does. The type
// is callable, so time.struct_time((y, m, d, ...)) constructs a value.
var timeStructTimeType = objects.NewStructSeqType(
	"struct_time", "time.struct_time",
	[]string{
		"tm_year", "tm_mon", "tm_mday", "tm_hour", "tm_min", "tm_sec",
		"tm_wday", "tm_yday", "tm_isdst", "tm_zone", "tm_gmtoff",
	},
	9, 0,
)

// English C-locale names. CPython's strftime reads the current locale, but with
// no setlocale call the process stays in the C locale, so these are what the
// darwin oracle emits and what every host must reproduce.
var (
	weekdayAbbr = [7]string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	weekdayFull = [7]string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}
	monthAbbr   = [12]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	monthFull   = [12]string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"}
)

// timeStrftime is time.strftime(format[, t]): render a struct_time or nine-tuple
// through the C strftime directives. With no time given CPython reads the
// current localtime, but every caller the floor drives passes one explicitly, so
// the missing form is a plain error rather than a host-dependent clock read.
func timeStrftime(args []objects.Object) (objects.Object, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, objects.Raise(objects.TypeError, "strftime() takes at most 2 arguments (%d given)", len(args))
	}
	format, ok := objects.AsStr(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "strftime() argument 1 must be str, not %s", args[0].TypeName())
	}
	if len(args) == 1 {
		return nil, objects.Raise(objects.TypeError, "strftime() requires a time tuple")
	}
	tm, err := timeReadTuple(args[1])
	if err != nil {
		return nil, err
	}
	return objects.NewStr(strftimeFormat(format, tm)), nil
}

// timeTuple is the nine calendar fields strftime reads, in struct_time order.
type timeTuple struct {
	year, mon, mday, hour, min, sec, wday, yday, isdst int
}

// timeReadTuple drains the first nine items of a struct_time, tuple or list into
// the calendar fields, raising CPython's TypeError when the sequence is short.
func timeReadTuple(o objects.Object) (timeTuple, error) {
	var t timeTuple
	items, err := objects.IterToSlice(o)
	if err != nil {
		return t, objects.Raise(objects.TypeError, "Tuple or struct_time argument required")
	}
	if len(items) < 9 {
		return t, objects.Raise(objects.TypeError, "function takes at least 9 arguments (%d given)", len(items))
	}
	fields := []*int{&t.year, &t.mon, &t.mday, &t.hour, &t.min, &t.sec, &t.wday, &t.yday, &t.isdst}
	for i, p := range fields {
		v, ok := objects.AsInt(items[i])
		if !ok {
			return t, objects.Raise(objects.TypeError, "an integer is required (got type %s)", items[i].TypeName())
		}
		*p = int(v)
	}
	return t, nil
}

// strftimeFormat walks the format string and expands each %directive from the
// calendar fields, matching the C strftime the oracle runs. An unknown directive
// passes through as the literal percent and letter, which is what BSD strftime
// does with a code it does not know.
func strftimeFormat(format string, t timeTuple) string {
	var b strings.Builder
	rs := []rune(format)
	for i := 0; i < len(rs); i++ {
		if rs[i] != '%' || i+1 >= len(rs) {
			b.WriteRune(rs[i])
			continue
		}
		i++
		b.WriteString(strftimeDirective(rs[i], t))
	}
	return b.String()
}

// strftimeDirective renders one directive. The weekday math uses two views of
// tm_wday: Python's Monday-zero index for the names and %u, and the Sunday-zero
// index C uses for %w and the %U week count.
func strftimeDirective(d rune, t timeTuple) string {
	wdaySun0 := (t.wday + 1) % 7 // 0 = Sunday, the C tm_wday convention
	switch d {
	case '%':
		return "%"
	case 'n':
		return "\n"
	case 't':
		return "\t"
	case 'a':
		return weekdayAbbr[t.wday%7]
	case 'A':
		return weekdayFull[t.wday%7]
	case 'b', 'h':
		return monthAbbr[(t.mon-1)%12]
	case 'B':
		return monthFull[(t.mon-1)%12]
	case 'Y':
		return fmt.Sprintf("%04d", t.year)
	case 'y':
		return fmt.Sprintf("%02d", ((t.year%100)+100)%100)
	case 'C':
		return fmt.Sprintf("%02d", t.year/100)
	case 'm':
		return fmt.Sprintf("%02d", t.mon)
	case 'd':
		return fmt.Sprintf("%02d", t.mday)
	case 'e':
		return fmt.Sprintf("%2d", t.mday)
	case 'H':
		return fmt.Sprintf("%02d", t.hour)
	case 'I':
		return fmt.Sprintf("%02d", hour12(t.hour))
	case 'M':
		return fmt.Sprintf("%02d", t.min)
	case 'S':
		return fmt.Sprintf("%02d", t.sec)
	case 'j':
		return fmt.Sprintf("%03d", t.yday)
	case 'p':
		if t.hour < 12 {
			return "AM"
		}
		return "PM"
	case 'w':
		return fmt.Sprintf("%d", wdaySun0)
	case 'u':
		return fmt.Sprintf("%d", t.wday+1)
	case 'U':
		return fmt.Sprintf("%02d", (t.yday-1+7-wdaySun0)/7)
	case 'W':
		return fmt.Sprintf("%02d", (t.yday-1+7-t.wday)/7)
	case 'V':
		_, week := isoYearWeek(t)
		return fmt.Sprintf("%02d", week)
	case 'G':
		year, _ := isoYearWeek(t)
		return fmt.Sprintf("%04d", year)
	case 'c':
		return fmt.Sprintf("%s %s %2d %02d:%02d:%02d %04d",
			weekdayAbbr[t.wday%7], monthAbbr[(t.mon-1)%12], t.mday, t.hour, t.min, t.sec, t.year)
	case 'x':
		return fmt.Sprintf("%02d/%02d/%02d", t.mon, t.mday, ((t.year%100)+100)%100)
	case 'X':
		return fmt.Sprintf("%02d:%02d:%02d", t.hour, t.min, t.sec)
	case 'z':
		return "" // tm_gmtoff is None for the nine-tuple datetime builds
	case 'Z':
		return "" // tm_zone is None for the nine-tuple datetime builds
	}
	return "%" + string(d)
}

// hour12 maps a 24-hour clock to the 12-hour clock %I shows, so 0 and 12 both
// read 12.
func hour12(h int) int {
	h %= 12
	if h == 0 {
		return 12
	}
	return h
}

// isoYearWeek computes the ISO 8601 week-numbering year and week for %G and %V.
// Week 1 is the week holding the first Thursday, so early-January days can fall
// in the last week of the prior year and late-December days in week 1 of the
// next.
func isoYearWeek(t timeTuple) (int, int) {
	isoWeekday := t.wday + 1 // 1 = Monday .. 7 = Sunday
	week := (t.yday - isoWeekday + 10) / 7
	year := t.year
	if week < 1 {
		year--
		week = isoWeeksInYear(year)
	} else if week > isoWeeksInYear(year) {
		year++
		week = 1
	}
	return year, week
}

// isoWeeksInYear is 53 when the ISO year is a long year and 52 otherwise, by the
// standard first-weekday test.
func isoWeeksInYear(y int) int {
	if isoP(y) == 4 || isoP(y-1) == 3 {
		return 53
	}
	return 52
}

// isoP is the weekday of 31 December in the proleptic Gregorian calendar, the
// helper the long-year test is phrased in.
func isoP(y int) int {
	return ((y+y/4-y/100+y/400)%7 + 7) % 7
}
