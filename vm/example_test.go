// Copyright 2025 Splunk Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package vm_test

import (
	"fmt"

	"github.com/splunk/qbec/vm"
)

func Example() {
	jvm := vm.New(vm.Config{})

	code := `
function (str, num) {
	foo: str,
	bar: num,
	baz: std.extVar('baz'),
}
`
	vs := vm.VariableSet{}.
		WithTopLevelVars(
			vm.NewVar("str", "hello"),
			vm.NewCodeVar("num", "10"),
		).
		WithVars(
			vm.NewVar("baz", "world"),
		)

	out, err := jvm.EvalCode("inline-code.jsonnet", vm.MakeCode(code), vs)
	if err != nil {
		panic(err)
	}

	fmt.Println(out)
	// Output:
	// {
	//    "bar": 10,
	//    "baz": "world",
	//    "foo": "hello"
	// }
}
