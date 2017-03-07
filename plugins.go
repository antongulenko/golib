package golib

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const gopathPluginDir = "bin"

// PluginSearchPath returns a list of directories that can be used to search for plugins.
// It contains all directories from the PATH environment variable, all bin/ subdirectories of
// the GOPATH environment variable, the current working directory and the directory of the
// current executable. The result is sorted and does not contain duplicates.
func PluginSearchPath() ([]string, error) {
	// Search all PATH directories
	paths := strings.Split(os.Getenv("PATH"), string(os.PathListSeparator))

	// Search all bin/ directories in GOPATH
	for _, gopath := range strings.Split(os.Getenv("GOPATH"), string(os.PathListSeparator)) {
		paths = append(paths, path.Join(gopath, gopathPluginDir))
	}

	// Search the current working directory
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	paths = append(paths, wd)

	// Search the directory of the current executable
	executableDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return nil, err
	}
	paths = append(paths, executableDir)

	return RemoveDuplicates(paths), nil
}

// FindPrefixedFiles reads the contents of all given directories and returns a list of
// files with a basename that matches the given regex. It is intended to find
// plugin executables. All directories are processed, and a list of all encountered errors is returned.
func FindMatchingFiles(regex *regexp.Regexp, directories []string) (result []string, errs []error) {
	for _, dir := range directories {
		files, readErr := ioutil.ReadDir(dir)
		if readErr != nil {
			errs = append(errs, readErr)
			continue
		}
		for _, file := range files {
			if file.IsDir() {
				continue
			}
			baseName := file.Name()
			if regex.MatchString(baseName) {
				result = append(result, path.Join(dir, baseName))
			}
		}
	}
	return
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
