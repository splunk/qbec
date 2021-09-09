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
