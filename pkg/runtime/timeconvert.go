package runtime

import (
	"fmt"
	"math"
	"time"

	"github.com/tamnd/unagi/pkg/objects"
)

// The epoch conversions the pure-Python datetime reaches for its clock-facing
// constructors: gmtime and localtime break a timestamp into a struct_time, and
// mktime folds a local struct_time back to a timestamp. gmtime is UTC and so
// host independent; localtime and mktime read the host zone, which is why the
// conformance golden only exercises gmtime and the mktime/localtime round trip.

// timeGmtime is time.gmtime([secs]): the UTC breakdown of a timestamp, with
// tm_zone fixed to UTC and tm_gmtoff to zero, matching CPython.
func timeGmtime(args []objects.Object) (objects.Object, error) {
	secs, err := timeSecsArg("gmtime", args)
	if err != nil {
		return nil, err
	}
	return structTimeFromGoTime(time.Unix(secs, 0).UTC(), "UTC", 0, 0), nil
}

// timeLocaltime is time.localtime([secs]): the same breakdown in the host's
// local zone, with tm_zone and tm_gmtoff read from that zone and tm_isdst set
// when the zone is in daylight saving.
func timeLocaltime(args []objects.Object) (objects.Object, error) {
	secs, err := timeSecsArg("localtime", args)
	if err != nil {
		return nil, err
	}
	local := time.Unix(secs, 0).Local()
	zone, offset := local.Zone()
	dst := 0
	if local.IsDST() {
		dst = 1
	}
	return structTimeFromGoTime(local, zone, offset, dst), nil
}

// timeMktime is time.mktime(t): the inverse of localtime, reading a nine-field
// struct_time as local wall-clock time and returning the epoch seconds as a
// float. Go resolves the zone offset and any daylight saving for the date.
func timeMktime(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "mktime() takes exactly one argument (%d given)", len(args))
	}
	tm, err := timeReadTuple(args[0])
	if err != nil {
		return nil, err
	}
	t := time.Date(tm.year, time.Month(tm.mon), tm.mday, tm.hour, tm.min, tm.sec, 0, time.Local)
	return objects.NewFloat(float64(t.Unix())), nil
}

// timeAsctime is time.asctime([t]): the fixed 24-character rendering of a
// struct_time, the same shape as strftime's %c. With no argument CPython reads
// the current localtime, which the floor never does, so that form is an error.
func timeAsctime(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "asctime() takes at most 1 argument (%d given)", len(args))
	}
	tm, err := timeReadTuple(args[0])
	if err != nil {
		return nil, err
	}
	return objects.NewStr(asctimeString(tm)), nil
}

// timeCtime is time.ctime([secs]): asctime of the local breakdown of a
// timestamp, host dependent through the local zone.
func timeCtime(args []objects.Object) (objects.Object, error) {
	secs, err := timeSecsArg("ctime", args)
	if err != nil {
		return nil, err
	}
	local := time.Unix(secs, 0).Local()
	zone, offset := local.Zone()
	dst := 0
	if local.IsDST() {
		dst = 1
	}
	st := structTimeFromGoTime(local, zone, offset, dst)
	tm, err := timeReadTuple(st)
	if err != nil {
		return nil, err
	}
	return objects.NewStr(asctimeString(tm)), nil
}

// asctimeString renders the "Www Mmm dd hh:mm:ss yyyy" form, the day space
// padded to two columns as %e and %c produce it.
func asctimeString(t timeTuple) string {
	return fmt.Sprintf("%s %s %2d %02d:%02d:%02d %04d",
		weekdayAbbr[t.wday%7], monthAbbr[(t.mon-1)%12], t.mday, t.hour, t.min, t.sec, t.year)
}

// structTimeFromGoTime builds a struct_time from a Go time already placed in the
// wanted zone, converting Go's Sunday-zero weekday to Python's Monday-zero one
// and filling the two named-only zone fields.
func structTimeFromGoTime(t time.Time, zone string, offset, dst int) objects.Object {
	pyWday := (int(t.Weekday()) + 6) % 7 // Go counts Sunday as 0, Python Monday as 0
	seq := []objects.Object{
		objects.NewInt(int64(t.Year())),
		objects.NewInt(int64(t.Month())),
		objects.NewInt(int64(t.Day())),
		objects.NewInt(int64(t.Hour())),
		objects.NewInt(int64(t.Minute())),
		objects.NewInt(int64(t.Second())),
		objects.NewInt(int64(pyWday)),
		objects.NewInt(int64(t.YearDay())),
		objects.NewInt(int64(dst)),
	}
	vals := make([]objects.Object, len(seq)+2)
	copy(vals, seq)
	vals[9] = objects.NewStr(zone)
	vals[10] = objects.NewInt(int64(offset))
	return timeStructTimeType.NewStructSeq(seq, vals)
}

// timeSecsArg reads the optional timestamp shared by gmtime, localtime and
// ctime: absent or None means the current time, and a non-number raises the
// CPython TypeError. The fractional part is dropped, since struct_time carries
// only whole seconds.
func timeSecsArg(name string, args []objects.Object) (int64, error) {
	if len(args) > 1 {
		return 0, objects.Raise(objects.TypeError, "%s() takes at most 1 argument (%d given)", name, len(args))
	}
	if len(args) == 0 || args[0] == objects.None {
		return time.Now().Unix(), nil
	}
	f, ok := objects.AsFloat(args[0])
	if !ok {
		return 0, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", args[0].TypeName())
	}
	return int64(math.Floor(f)), nil
}
