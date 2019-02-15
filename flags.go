package golib

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"strings"
)

// Flags is a bit-mask type used in the RegisterFlags() function.
type Flags uint32

const (
	// FlagsAll makes RegisterFlags() enable all available flags.
	FlagsAll = 0xffffffff

	// FlagsLog enables flags that configure the logger (Package github.com/sirupsen/logrus).
	FlagsLog = 1 << iota

	// FlagsProfile enables flags the configure profiling of CPU and memory.
	FlagsProfile

	// FlagsTasks enables flags that help debugging the shutdown sequence Tasks and TaskGroups.
	FlagsTasks
)

// RegisterFlags registers various flags provided by the golib package, controlled
// by the bit-mask parameter.
func RegisterFlags(flags Flags) {
	if flags&FlagsLog != 0 {
		RegisterLogFlags()
	}
	if flags&FlagsProfile != 0 {
		RegisterProfileFlags()
	}
	if flags&FlagsTasks != 0 {
		RegisterTaskFlags()
	}
}

// StringSlice implements the flag.Value interface and stores every occurrence
// of the according flag in one string slice.
type StringSlice []string

// String implements the flag.Value interface by printing the contents of the underlying string slice.
func (i *StringSlice) String() string {
	return fmt.Sprintf("%v", *i)
}

// Set implements the flag.Value interface by adding the given string to the underlying string slice.
func (i *StringSlice) Set(value string) error {
	*i = append(*i, value)
	return nil
}

// KeyValueSeparator is used by KeyValueStringSlice to split 'key=value' parameters.
const KeyValueSeparator = "="

// KeyValueStringSlice implements the flag.Value interface. It expects value of the form 'key=value'
// and splits them into the corresponding parts.
type KeyValueStringSlice struct {
	Keys   []string
	Values []string
}

// String implements the flag.Value interface by printing all contains key-value pairs.
func (k *KeyValueStringSlice) String() string {
	return FormatOrderedMap(k.Keys, k.Values)
}

// Set implements the flag.Value interface by splitting the 'key=value' string
// and returning an error if the format is wrong.
func (k *KeyValueStringSlice) Set(value string) error {
	parts := strings.SplitN(value, KeyValueSeparator, 2)
	if len(parts) != 2 {
		return fmt.Errorf("Wrong format. Need key=value, got " + value)
	}
	k.Keys = append(k.Keys, parts[0])
	k.Values = append(k.Values, parts[1])
	return nil
}

// Map returns a map-representation of the contained key-value pairs.
func (k *KeyValueStringSlice) Map() map[string]string {
	result := make(map[string]string, len(k.Keys))
	for i, key := range k.Keys {
		result[key] = k.Values[i]
	}
	return result
}

// Put sets the given value to the first instance of the given key. All other instances of the
// given key remain unchanged. If the key is not yet present in the receiver, the new key-value pair
// is appended.
func (k *KeyValueStringSlice) Put(key, value string) {
	for i, storedKey := range k.Keys {
		if storedKey == key {
			k.Values[i] = value
			return
		}
	}
	k.Keys = append(k.Keys, key)
	k.Values = append(k.Values, value)
}

// Delete deletes all instances of the given key from the receiving KeyValueStringSlice. If the key
// is not present, the receiver remains unchanged.
func (k *KeyValueStringSlice) Delete(key string) {
	for i := 0; i < len(k.Keys); i++ {
		if k.Keys[i] == key {
			k.Keys = k.deleteIndex(i, k.Keys)
			k.Values = k.deleteIndex(i, k.Values)
		}
	}
}

func (k *KeyValueStringSlice) deleteIndex(i int, slice []string) []string {
	copy(slice[i:], slice[i+1:])
	return slice[:len(slice)-1]
}

// FormatMap returns a readable representation of the given string map.
func FormatMap(m map[string]string) string {
	keys := make([]string, 0, len(m))
	values := make([]string, 0, len(m))
	for k, v := range m {
		keys = append(keys, k)
		values = append(values, v)
	}
	return FormatOrderedMap(keys, values)
}

// FormatOrderedMap returns a readable representation of the given key-value pairs.
func FormatOrderedMap(keys []string, values []string) string {
	var buf bytes.Buffer
	started := false
	for i, val := range values {
		key := keys[i]
		if started {
			buf.WriteString(", ")
		}
		buf.WriteString(key)
		buf.WriteString("=")
		buf.WriteString(val)
		started = true
	}
	return buf.String()
}

// EscapeExistingFlags can be used before defining new flags to escape existing flags that have been defined
// by other packages or modules. This can be used to avoid collisions of flag names.
func EscapeExistingFlags(prefix string) {
	oldCommandLine := flag.CommandLine
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	oldCommandLine.VisitAll(func(f *flag.Flag) {
		flag.Var(f.Value, prefix+f.Name, f.Usage)
	})
}

// When packages or modules are loaded AFTER parsing flags, avoid collisions when flags are re-defined.
func ParseFlags() []string {
	flag.Parse()
	args := flag.Args()
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	return args
}
