/*
   Copyright 2019 Splunk Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

// Package sio provides logging functions with optional colorization.
package sio

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

const esc = "\x1b["

const (
	codeReset    = esc + "0m"  // reset all attributes
	colorRed     = esc + "31m" // set foreground to red
	colorMagenta = esc + "35m" // set foreground to magenta
	attrBold     = esc + "1m"  // set bold
	attrDim      = esc + "2m"  // set dim
	unicodeX     = "\u2718"    // X mark
)

type colors struct {
	sync.RWMutex
	enabled bool
}

func (ce *colors) isEnabled() bool {
	ce.RLock()
	defer ce.RUnlock()
	return ce.enabled
}

func (ce *colors) set(flag bool) {
	ce.Lock()
	defer ce.Unlock()
	ce.enabled = flag
}

var ce = &colors{enabled: true}

// EnableColors enables or disables colored output
func EnableColors(flag bool) {
	ce.set(flag)
}

// ColorsEnabled returns if colored output is enabled
func ColorsEnabled() bool {
	return ce.isEnabled()
}

func startColors(codes ...string) {
	if ce.isEnabled() {
		fmt.Fprint(Output, strings.Join(codes, ""))
	}
}

func reset() {
	if ce.isEnabled() {
		fmt.Fprint(Output, codeReset)
	}
}

// Output is the writer to which all prompts, errors and messages go.
// This is set to standard error by default.
var Output io.Writer = os.Stderr

// Println prints the supplied arguments to the standard writer
func Println(args ...interface{}) {
	fmt.Fprintln(Output, args...)
}

// Printf prints the supplied arguments to the standard writer.
func Printf(format string, args ...interface{}) {
	fmt.Fprintf(Output, format, args...)
}

// Noticeln prints the supplied arguments in a way that they will be noticed.
// Use sparingly.
func Noticeln(args ...interface{}) {
	startColors(attrBold)
	fmt.Fprintln(Output, args...)
	reset()
}

// Noticef prints the supplied arguments in a way that they will be noticed.
// Use sparingly.
func Noticef(format string, args ...interface{}) {
	startColors(attrBold)
	fmt.Fprintf(Output, format, args...)
	reset()
}

// Debugln prints the supplied arguments to the standard writer, de-emphasized
func Debugln(args ...interface{}) {
	startColors(attrDim)
	fmt.Fprintln(Output, args...)
	reset()
}

// Debugf prints the supplied arguments to the standard writer, de-emphasized.
func Debugf(format string, args ...interface{}) {
	startColors(attrDim)
	fmt.Fprintf(Output, format, args...)
	reset()
}

// Warnln prints the supplied arguments to the standard writer
// with some indication for a warning.
func Warnln(args ...interface{}) {
	startColors(colorMagenta, attrBold)
	fmt.Fprint(Output, "[warn] ")
	fmt.Fprintln(Output, args...)
	reset()
}

// Warnf prints the supplied arguments to the standard writer
// with some indication for a warning.
func Warnf(format string, args ...interface{}) {
	startColors(colorMagenta, attrBold)
	fmt.Fprint(Output, "[warn] ")
	fmt.Fprintf(Output, format, args...)
	reset()
}

// Errorln prints the supplied arguments to the standard writer
// with some indication that an error has occurred.
func Errorln(args ...interface{}) {
	startColors(colorRed, attrBold)
	fmt.Fprint(Output, unicodeX+" ")
	fmt.Fprintln(Output, args...)
	reset()
}

// Errorf prints the supplied arguments to the standard writer
// with some indication that an error has occurred.
func Errorf(format string, args ...interface{}) {
	startColors(colorRed, attrBold)
	fmt.Fprint(Output, unicodeX+" ")
	fmt.Fprintf(Output, format, args...)
	reset()
}
