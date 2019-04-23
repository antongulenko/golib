package golib

import (
	"bytes"
	"sort"

	"github.com/lunixbochs/vtclean"
	"golang.org/x/text/width"
)

// StringLength returns the number of normalized utf8-runes within the cleaned string.
// Clean means the string is stripped of terminal escape characters and color codes.
func StringLength(str string) (strlen int) {
	str = vtclean.Clean(str, false)
	for str != "" {
		_, rest, runeWidth := ReadRune(str)
		str = rest
		strlen += runeWidth
	}
	// Alternative:
	// strlen = utf8.RuneCountInString(str)
	// Alternative:
	// var ia norm.Iter
	// ia.InitString(norm.NFKD, str)
	return
}

// ReadRune reads one utf8 rune from the input string and provides information
// about the character width of the read rune.
func ReadRune(input string) (theRune string, rest string, runeWidth int) {
	prop, size := width.LookupString(input)
	rest = input[size:]
	theRune = input[:size]
	switch prop.Kind() {
	case width.EastAsianFullwidth, width.EastAsianHalfwidth, width.EastAsianWide:
		runeWidth = 2
	default:
		runeWidth = 1
	}
	return
}

// Substring returns a substring of the given input string, but the indices
// iFrom and iTo point to normalized utf8-runes within the cleaned string.
// Clean means the string is stripped of terminal escape characters and color codes.
// The total number of normalized utf8 runes in the clean string can be obtained from
// the StringLength() function.
// All runes and special codes will be preserved in the output string.
func Substring(str string, iFrom int, iTo int) string {

	// Find the start in the input string
	buf := bytes.NewBuffer(make([]byte, 0, len(str)))
	textWidth := 0
	cleanedLen := 0
	for str != "" && textWidth < iFrom {
		runeStr, rest, runeWidth := ReadRune(str)
		buf.Write([]byte(runeStr))

		cleaned := vtclean.Clean(buf.String(), false)
		if len(cleaned) > cleanedLen {
			// A visible rune was added
			cleanedLen = len(cleaned)
			textWidth += runeWidth
		}
		if textWidth >= iFrom {
			break
		}
		str = rest
	}

	iLen := iTo - iFrom
	possibleColor := false

	// Find the end in the input string
	to := 0
	suffix := []byte(str)
	buf.Reset()
	textWidth = 0
	cleanedLen = 0
	for str != "" && textWidth < iLen {
		runeStr, rest, runeWidth := ReadRune(str)
		buf.Write([]byte(runeStr))

		cleaned := vtclean.Clean(buf.String(), false)
		if len(cleaned) > cleanedLen {
			// A visible rune was added
			cleanedLen = len(cleaned)
			textWidth += runeWidth
		}
		if len(cleaned) != buf.Len() {
			// Contains invisible escape characters, possibly color codes
			possibleColor = true
		}
		if textWidth > iLen {
			if runeWidth == 2 {
				// Splitting wide character in the end, pad with a space
				suffix[to] = ' '
				to++
			}
			break
		}
		str = rest
		to += len(runeStr)
	}

	result := string(suffix[:to])
	if possibleColor {
		// Might contain color codes, make sure to disable colors at the end
		result += "\033[0m"
	}
	return result
}

func EqualStrings(a, b []string) bool {
	switch {
	case len(a) != len(b):
		return false
	case len(a) == 0:
		return true
	case &a[0] == &b[0]:
		// Compare the address of the array backing the slices
		return true
	}
	// Last resort: compare every string pair
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// RemoveDuplicates sorts the given string slice and returns a copy with all duplicate
// strings removed.
func RemoveDuplicates(strings []string) []string {
	sort.Strings(strings)
	result := make([]string, 0, len(strings))
	for _, str := range strings {
		if len(result) == 0 || str != result[len(result)-1] {
			result = append(result, str)
		}
	}
	return result
}
