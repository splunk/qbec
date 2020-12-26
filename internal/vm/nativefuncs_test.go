// Copyright 2017 The kubecfg authors
//
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package vm

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/google/go-jsonnet"
)

// copied from original code at https://github.com/ksonnet/kubecfg/blob/master/utils/nativefuncs_test.go
// and modified for use.

// check there is no err, and a == b.
func check(t *testing.T, err error, actual, expected string) {
	if err != nil {
		t.Errorf("Expected %q, got error: %q", expected, err.Error())
	} else if actual != expected {
		t.Errorf("Expected %q, got %q", expected, actual)
	}
}

func TestParseJson(t *testing.T) {
	vm := jsonnet.MakeVM()
	registerNativeFuncs(vm)

	_, err := vm.EvaluateSnippet("failtest", `std.native("parseJson")("barf{")`)
	if err == nil {
		t.Errorf("parseJson succeeded on invalid json")
	}

	x, err := vm.EvaluateSnippet("test", `std.native("parseJson")("null")`)
	check(t, err, x, "null\n")

	x, err = vm.EvaluateSnippet("test", `
    local a = std.native("parseJson")('{"foo": 3, "bar": 4}');
    a.foo + a.bar`)
	check(t, err, x, "7\n")
}

func TestParseYaml(t *testing.T) {
	vm := jsonnet.MakeVM()
	registerNativeFuncs(vm)

	_, err := vm.EvaluateSnippet("failtest", `std.native("parseYaml")("[barf")`)
	if err == nil {
		t.Errorf("parseYaml succeeded on invalid yaml")
	}

	x, err := vm.EvaluateSnippet("test", `std.native("parseYaml")("")`)
	check(t, err, x, "[ ]\n")

	x, err = vm.EvaluateSnippet("test", `
    local a = std.native("parseYaml")("foo:\n- 3\n- 4\n")[0];
    a.foo[0] + a.foo[1]`)
	check(t, err, x, "7\n")

	x, err = vm.EvaluateSnippet("test", `
    local a = std.native("parseYaml")("---\nhello\n---\nworld");
    a[0] + a[1]`)
	check(t, err, x, "\"helloworld\"\n")
}

func TestRegexMatch(t *testing.T) {
	vm := jsonnet.MakeVM()
	registerNativeFuncs(vm)

	_, err := vm.EvaluateSnippet("failtest", `std.native("regexMatch")("[f", "foo")`)
	if err == nil {
		t.Errorf("regexMatch succeeded with invalid regex")
	}

	x, err := vm.EvaluateSnippet("test", `std.native("regexMatch")("foo.*", "seafood")`)
	check(t, err, x, "true\n")

	x, err = vm.EvaluateSnippet("test", `std.native("regexMatch")("bar.*", "seafood")`)
	check(t, err, x, "false\n")
}

func TestRegexSubst(t *testing.T) {
	vm := jsonnet.MakeVM()
	registerNativeFuncs(vm)

	_, err := vm.EvaluateSnippet("failtest", `std.native("regexSubst")("[f", "foo", "bar")`)
	if err == nil {
		t.Errorf("regexSubst succeeded with invalid regex")
	}

	x, err := vm.EvaluateSnippet("test", `std.native("regexSubst")("a(x*)b", "-ab-axxb-", "T")`)
	check(t, err, x, "\"-T-T-\"\n")

	x, err = vm.EvaluateSnippet("test", `std.native("regexSubst")("a(x*)b", "-ab-axxb-", "${1}W")`)
	check(t, err, x, "\"-W-xxW-\"\n")
}

func TestRegexQuoteMeta(t *testing.T) {
	vm := jsonnet.MakeVM()
	registerNativeFuncs(vm)
	x, err := vm.EvaluateSnippet("test", `std.native("escapeStringRegex")("[f]")`)
	check(t, err, x, `"\\[f\\]"`+"\n")
}

func TestLabelSelectorMatch(t *testing.T) {
	vm := jsonnet.MakeVM()
	registerNativeFuncs(vm)
	tests := []struct {
		name     string
		selector string
		expected string
	}{
		{
			name:     "presence",
			selector: "env",
			expected: "yes",
		},
		{
			name:     "absence",
			selector: "!env",
			expected: "no",
		},
		{
			name:     "and-presence",
			selector: "env,region",
			expected: "yes",
		},
		{
			name:     "no-presence",
			selector: "foo",
			expected: "no",
		},
		{
			name:     "equality",
			selector: "region=us-west",
			expected: "yes",
		},
		{
			name:     "equality-no-match",
			selector: "env=prod",
			expected: "no",
		},
		{
			name:     "and",
			selector: "region=us-west,env=dev",
			expected: "yes",
		},
		{
			name:     "and-no-match",
			selector: "region=us-west,!env",
			expected: "no",
		},
		{
			name:     "in",
			selector: "env in (prod, dev)",
			expected: "yes",
		},
		{
			name:     "not-in",
			selector: "env notin (prod, dev)",
			expected: "no",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code := fmt.Sprintf(`
			local labels = { env: 'dev', region: 'us-west' };
			if std.native('labelsMatchSelector')(labels, '%s') then 'yes' else 'no'
`, test.selector)
			ret, err := vm.EvaluateSnippet("test.jsonnet", code)
			check(t, err, ret, fmt.Sprintf(`"%s"`+"\n", test.expected))
		})
	}
}

func TestLabelSelectorNegative(t *testing.T) {
	vm := jsonnet.MakeVM()
	registerNativeFuncs(vm)
	tests := []struct {
		name     string
		code     string
		errMatch *regexp.Regexp
	}{
		{
			name:     "bad map",
			code:     `std.native('labelsMatchSelector')([],'foo')`,
			errMatch: regexp.MustCompile(`invalid labels type, \[\]interface {}, want a map`),
		},
		{
			name:     "non-string map",
			code:     `std.native('labelsMatchSelector')({ foo: {} },'foo')`,
			errMatch: regexp.MustCompile(`invalid label map value, map\[string\]interface {}, want a string`),
		},
		{
			name:     "bad selector type",
			code:     `std.native('labelsMatchSelector')({},{})`,
			errMatch: regexp.MustCompile(`invalid selector of type map\[string\]interface {}, want a string`),
		},
		{
			name:     "bad selector",
			code:     `std.native('labelsMatchSelector')({},'!!env')`,
			errMatch: regexp.MustCompile(`invalid label selector: '!!env'`),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := vm.EvaluateSnippet("test.jsonnet", test.code)
			if err == nil {
				t.Errorf("labelsMatchSelector succeeded on invalid input")
			}
			if !test.errMatch.MatchString(err.Error()) {
				t.Errorf("message %q does not match %v", err.Error(), test.errMatch)
			}
		})
	}

}
