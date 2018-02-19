package golib

import "time"

// Full reference time: Mon Jan 2 15:04:05.999999 MST 2006

const SimpleTimeLayout = "2006-01-02 15:04:05"

// Call time.Parse(), and panic if there is a non-nil error.
// Intended for static initializers with proven correct input values, similar to regexp.MustCompile.
func ParseTime(layout string, timeStr string) time.Time {
	res, err := time.Parse(layout, timeStr)
	if err != nil {
		panic(err)
	}
	return res
}
