package golib

import (
	"bytes"
	"fmt"
)

// MultiError is a helper type for combining multiple error values into one.
type MultiError []error

// NilOrError returns either nil, the only contained error, or the entire MultiError,
// depending on the number of contained errors.
func (err MultiError) NilOrError() error {
	if len(err) == 0 {
		return nil
	} else if len(err) == 1 {
		return err[0]
	}
	return err
}

// Add adds the given error to the MultiError, if it is not nil.
func (err *MultiError) Add(errOrNil error) {
	if err != nil && errOrNil != nil {
		*err = append(*err, errOrNil)
	}
}

// Error implements the error interface by printing all contained errors
// on a separate line.
func (err MultiError) Error() string {
	switch len(err) {
	case 0:
		return "No error"
	case 1:
		return err[0].Error()
	default:
		var buf bytes.Buffer
		fmt.Fprintf(&buf, "Multiple errors:\n")
		for i, e := range err {
			if i > 0 {
				fmt.Fprintf(&buf, "\n")
			}
			fmt.Fprintf(&buf, "\t%v. %v", i+1, e)
		}
		return buf.String()
	}
}
