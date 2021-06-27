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

package sio

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOutputWithoutColors(t *testing.T) {
	var buf bytes.Buffer
	orig := Output
	origC := ColorsEnabled()
	defer func() { Output = orig; EnableColors(origC) }()
	EnableColors(false)
	Output = &buf

	Println("this", "is", "a", "message")
	Warnln("this", "is", "a", "warning")
	Errorln("this", "is", "an", "error")
	Noticeln("this", "is", "a", "notice")
	Debugln("this", "is", "an", "extra")

	Printf("This is %s %s\n", "a", "message")
	Warnf("This is %s %s\n", "a", "warning")
	Errorf("This is %s %s\n", "an", "error")
	Noticef("This is %s %s\n", "a", "notice")
	Debugf("This is %s %s\n", "an", "extra")

	a := assert.New(t)
	lines := strings.Split(buf.String(), "\n")
	a.Contains(lines, "this is a message")
	a.Contains(lines, "this is a notice")
	a.Contains(lines, "this is an extra")
	a.Contains(lines, "[warn] this is a warning")
	a.Contains(lines, unicodeX+" this is an error")
	a.Contains(lines, "This is a message")
	a.Contains(lines, "This is a notice")
	a.Contains(lines, "This is an extra")
	a.Contains(lines, "[warn] This is a warning")
	a.Contains(lines, unicodeX+" This is an error")
}

func TestOutputWithColors(t *testing.T) {
	var buf bytes.Buffer
	orig := Output
	origC := ColorsEnabled()
	defer func() { Output = orig; EnableColors(origC) }()
	EnableColors(true)
	Output = &buf

	Println("this", "is", "a", "message")
	Noticeln("this", "is", "a", "notice")
	Warnln("this", "is", "a", "warning")
	Errorln("this", "is", "an", "error")
	Debugln("this", "is", "an", "extra")

	Printf("This is %s %s\n", "a", "message")
	Noticef("This is %s %s\n", "a", "notice")
	Warnf("This is %s %s\n", "a", "warning")
	Errorf("This is %s %s\n", "an", "error")
	Debugf("This is %s %s\n", "an", "extra")

	a := assert.New(t)
	s := buf.String()
	a.Contains(s, "this is a message\n")
	a.Contains(s, attrBold+"this is a notice\n"+codeReset)
	a.Contains(s, colorMagenta+attrBold+"[warn] this is a warning\n"+codeReset)
	a.Contains(s, colorRed+attrBold+unicodeX+" this is an error\n"+codeReset)
	a.Contains(s, attrDim+"this is an extra\n"+codeReset)
	a.Contains(s, "This is a message\n")
	a.Contains(s, attrBold+"This is a notice\n"+codeReset)
	a.Contains(s, attrDim+"This is an extra\n"+codeReset)
	a.Contains(s, colorMagenta+attrBold+"[warn] This is a warning\n"+codeReset)
	a.Contains(s, colorRed+attrBold+unicodeX+" This is an error\n"+codeReset)
}

func TestErrorString(t *testing.T) {
	origC := ColorsEnabled()
	defer func() { EnableColors(origC) }()
	EnableColors(true)
	x := ErrorString("test")
	a := assert.New(t)
	a.Contains(x, attrBold)
	a.Contains(x, colorRed)
	a.Contains(x, codeReset)

	EnableColors(false)
	x = ErrorString("test")
	a.NotContains(x, colorRed)
}
