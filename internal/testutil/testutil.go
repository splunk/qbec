package testutil

import (
	"runtime"
)

// FileNotFoundMessage is the string to be used in test code for comparing to file not found error messages.
var FileNotFoundMessage = "no such file or directory"

func init() {
	if runtime.GOOS == "windows" {
		FileNotFoundMessage = "The system cannot find the file specified."
	}
}
