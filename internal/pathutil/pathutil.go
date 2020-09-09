package pathutil

import (
	"path/filepath"
	"runtime"
	"strings"
)

const (
	stdSep = "/"
	osSep  = string(filepath.Separator)
)

// FileNotFoundMessage is the string to be used in test code for comparing to file not found error messages.
var FileNotFoundMessage = "no such file or directory"

func init() {
	if runtime.GOOS == "windows" {
		FileNotFoundMessage = "The system cannot find the file specified."
	}
}

// ToOSPath returns the OS-specific path given a canonical path.
func ToOSPath(canonical string) string {
	return strings.Replace(canonical, stdSep, osSep, -1)
}

// ToCanonicalPath returns the canonical path given an OS path.
func ToCanonicalPath(osPath string) string {
	return strings.Replace(osPath, osSep, stdSep, -1)
}
