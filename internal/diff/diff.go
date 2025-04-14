// Copyright 2025 Splunk Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package diff contains primitives for diff-ing objects and strings.
package diff

import (
	"strings"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	godiff "github.com/pmezard/go-difflib/difflib"
)

const (
	escGreen = "\x1b[32m"
	escRed   = "\x1b[31m"
	escReset = "\x1b[0m"
)

// Options are options for the diff. The zero-value is valid.
// Use a negative number for the context if you really want 0 context lines.
type Options struct {
	LeftName  string // name of left side
	RightName string // name of right side
	Context   int    // number of context lines in the diff, defaults to 3
	Colorize  bool   // added colors to the diff
}

// Strings diffs the left and right strings and returns
// the diff. A zero-length slice is returned when there are no diffs.
func Strings(left, right string, opts Options) ([]byte, error) {
	if opts.Context == 0 {
		opts.Context = 3
	}
	if opts.Context < 0 {
		opts.Context = 0
	}
	ud := godiff.UnifiedDiff{
		A:        godiff.SplitLines(left),
		B:        godiff.SplitLines(right),
		FromFile: opts.LeftName,
		ToFile:   opts.RightName,
		Context:  opts.Context,
	}
	s, err := godiff.GetUnifiedDiffString(ud)
	if err != nil {
		return nil, errors.Wrap(err, "diff error")
	}
	if opts.Colorize && len(s) > 0 {
		lines := godiff.SplitLines(s)
		var out []string
		for _, l := range lines {
			switch {
			case strings.HasPrefix(l, "-"):
				out = append(out, escRed+l+escReset)
			case strings.HasPrefix(l, "+"):
				out = append(out, escGreen+l+escReset)
			default:
				out = append(out, l)
			}
		}
		s = strings.Join(out, "")
	}
	return []byte(s), nil
}

// Objects renders the left and right objects passed to it as YAML and returns
// the diff. A zero-length slice is returned when there are no diffs.
func Objects(left, right interface{}, opts Options) ([]byte, error) {
	asYaml := func(data interface{}) ([]byte, error) {
		if data == nil {
			return []byte{}, nil
		}
		return yaml.Marshal(data)
	}
	l, err := asYaml(left)
	if err != nil {
		return nil, errors.Wrap(err, "marshal left")
	}
	r, err := asYaml(right)
	if err != nil {
		return nil, errors.Wrap(err, "marshal right")
	}
	return Strings(string(l), string(r), opts)
}
