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

package natives

// copied from original code at https://github.com/ksonnet/kubecfg/blob/master/utils/nativefuncs.go
// and modified for use.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"

	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/labels"
)

// Register adds qbec's native jsonnet functions to the provided VM
func Register(vm *jsonnet.VM) {
	// NB: libjsonnet native functions can only pass primitive
	// types, so some functions json-encode the arg.  These
	// "*FromJson" functions will be replaced by regular native
	// version when libjsonnet is able to support this.

	vm.NativeFunction(&jsonnet.NativeFunction{
		Name:   "parseJson",
		Params: []ast.Identifier{"json"},
		Func: func(args []interface{}) (res interface{}, err error) {
			str := args[0].(string)
			data := []byte(str)
			res, err = ParseJSON(bytes.NewReader(data))
			return
		},
	})

	vm.NativeFunction(&jsonnet.NativeFunction{
		Name:   "parseYaml",
		Params: []ast.Identifier{"yaml"},
		Func: func(args []interface{}) (res interface{}, err error) {
			data := []byte(args[0].(string))
			return ParseYAMLDocuments(bytes.NewReader(data))
		},
	})

	vm.NativeFunction(&jsonnet.NativeFunction{
		Name:   "renderYaml",
		Params: []ast.Identifier{"data"},
		Func: func(args []interface{}) (res interface{}, err error) {
			return RenderYAMLDocuments(args[0])
		},
	})

	vm.NativeFunction(&jsonnet.NativeFunction{
		Name:   "escapeStringRegex",
		Params: []ast.Identifier{"str"},
		Func: func(args []interface{}) (res interface{}, err error) {
			return regexp.QuoteMeta(args[0].(string)), nil
		},
	})

	vm.NativeFunction(&jsonnet.NativeFunction{
		Name:   "regexMatch",
		Params: []ast.Identifier{"regex", "string"},
		Func: func(args []interface{}) (res interface{}, err error) {
			return regexp.MatchString(args[0].(string), args[1].(string))
		},
	})

	vm.NativeFunction(&jsonnet.NativeFunction{
		Name:   "regexSubst",
		Params: []ast.Identifier{"regex", "src", "repl"},
		Func: func(args []interface{}) (res interface{}, err error) {
			regex := args[0].(string)
			src := args[1].(string)
			repl := args[2].(string)

			r, err := regexp.Compile(regex)
			if err != nil {
				return "", err
			}
			return r.ReplaceAllString(src, repl), nil
		},
	})

	vm.NativeFunction(&jsonnet.NativeFunction{
		Name:   "expandHelmTemplate",
		Params: []ast.Identifier{"chart", "values", "options"},
		Func: func(args []interface{}) (res interface{}, err error) {
			chart := args[0].(string)
			values := args[1].(map[string]interface{})
			options := args[2].(map[string]interface{})
			var h helmOptions
			b, err := json.Marshal(options)
			if err != nil {
				return nil, errors.Wrap(err, "marshal options to JSON")
			}
			if err := json.Unmarshal(b, &h); err != nil {
				return nil, errors.Wrap(err, "unmarshal options from JSON")
			}
			return expandHelmTemplate(chart, values, h)
		},
	})

	vm.NativeFunction(&jsonnet.NativeFunction{
		Name:   "labelsMatchSelector",
		Params: []ast.Identifier{"labels", "selectorString"},
		Func: func(args []interface{}) (res interface{}, err error) {
			lbls, ok := args[0].(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid labels type, %v, want a map", reflect.TypeOf(args[0]))
			}
			selStr, ok := args[1].(string)
			if !ok {
				return nil, fmt.Errorf("invalid selector of type %v, want a string", reflect.TypeOf(args[1]))
			}
			input := map[string]string{}
			for k, v := range lbls {
				val, ok := v.(string)
				if !ok {
					return nil, fmt.Errorf("invalid label map value, %v, want a string", reflect.TypeOf(v))
				}
				input[k] = val
			}
			sel, err := labels.Parse(selStr)
			if err != nil {
				return false, fmt.Errorf("invalid label selector: '%s', %v", selStr, err)
			}
			return sel.Matches(labels.Set(input)), nil
		},
	})
}
