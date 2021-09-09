package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"
	"github.com/pkg/errors"
	"github.com/splunk/qbec/vm"
	"github.com/splunk/qbec/vm/vmutil"
)

const (
	fldRoot           = "root"
	fldFile           = "file"
	fldCode           = "code"
	fldFormat         = "format"
	fldLibPaths       = "lib_paths"
	fldExtVars        = "ext_vars"
	fldExtCodeVars    = "ext_code_vars"
	fldTLAVars        = "tla_vars"
	fldTLACodeVars    = "tla_code_vars"
	fldComputedResult = "result"

	fmtJSON      = "json"
	fmtYAML      = "yaml"
	fmtMultiYAML = "multi-yaml"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: func() *schema.Provider {
			return &schema.Provider{
				DataSourcesMap: map[string]*schema.Resource{
					"qbec_eval": {
						Schema: map[string]*schema.Schema{
							fldComputedResult: {
								Type:     schema.TypeString,
								Computed: true,
							},
							fldRoot: {
								Type:        schema.TypeString,
								Required:    true,
								Description: "root directory for jsonnet evaluation",
							},
							fldFile: {
								Type:        schema.TypeString,
								Optional:    true,
								Description: "jsonnet file to evaluate, relative to the root directory",
							},
							fldCode: {
								Type:        schema.TypeString,
								Optional:    true,
								Description: "jsonnet code to evaluate",
							},
							fldFormat: {
								Type:        schema.TypeString,
								Description: "json (default) or yaml",
								Optional:    true,
								Default:     fmtJSON,
							},
							fldLibPaths: {
								Type:        schema.TypeList,
								Description: "library paths to use relative to the root directory",
								Elem: &schema.Schema{
									Type: schema.TypeString,
								},
								Optional: true,
							},
							fldExtVars: {
								Type:        schema.TypeMap,
								Description: "external variables to set as strings",
								Elem:        &schema.Schema{Type: schema.TypeString},
								Optional:    true,
							},
							fldExtCodeVars: {
								Type:        schema.TypeMap,
								Description: "external variables to set as code variables",
								Elem:        &schema.Schema{Type: schema.TypeString},
								Optional:    true,
							},
							fldTLAVars: {
								Type:        schema.TypeMap,
								Description: "TLA variables to set as strings",
								Elem:        &schema.Schema{Type: schema.TypeString},
								Optional:    true,
							},
							fldTLACodeVars: {
								Type:        schema.TypeMap,
								Description: "TLA variables to set as code variables",
								Elem:        &schema.Schema{Type: schema.TypeString},
								Optional:    true,
							},
						},
						ReadContext: evalJsonnet,
					},
				},
			}
		},
	})
}

// since we need to change the workdir, only one eval can happen at any one time :(
var globalLock sync.Mutex

func evalJsonnet(_ context.Context, data *schema.ResourceData, _ interface{}) (res diag.Diagnostics) {
	globalLock.Lock()
	defer globalLock.Unlock()

	{
		root := data.Get(fldRoot).(string)
		wd, err := os.Getwd()
		if err != nil {
			return diag.FromErr(errors.Wrap(err, "get working directory"))
		}
		if err := os.Chdir(root); err != nil {
			return diag.FromErr(errors.Wrapf(err, "change working directory to %s", root))
		}
		defer func() { _ = os.Chdir(wd) }()
	}

	var file, code string

	{
		file0, fileFound := data.GetOk("file")
		code0, codeFound := data.GetOk("code")

		switch {
		case !fileFound && !codeFound:
			return diag.FromErr(fmt.Errorf("one of '%s' or '%s' attributes must be set", fldCode, fldFile))
		case fileFound && codeFound:
			return diag.FromErr(fmt.Errorf("cannot set both '%s' and '%s'", fldCode, fldFile))
		case fileFound:
			file = file0.(string)
		default:
			code = code0.(string)
			if strings.Trim(code, " \t\n\r") == "" {
				return diag.FromErr(fmt.Errorf("%s string is empty", fldCode))
			}
		}
	}

	var format string
	{
		format0, ok := data.GetOk(fldFormat)
		if !ok {
			format = fmtJSON
		} else {
			format = format0.(string)
		}
		if format == "" {
			format = fmtJSON
		}
		switch format {
		case fmtJSON, fmtYAML, fmtMultiYAML:
		default:
			return diag.FromErr(fmt.Errorf("invalid %s field, want be one of %s, %s, or %s, got %s", fldFormat, fmtJSON, fmtYAML, fmtMultiYAML, format))
		}
	}

	libPaths, err := getLibPaths(data)
	if err != nil {
		return diag.FromErr(err)
	}
	vars, err := getVariables(data)
	if err != nil {
		return diag.FromErr(err)
	}

	jvm := vm.New(vm.Config{
		LibPaths: libPaths,
	})

	var jsonString string

	if file != "" {
		jsonString, err = jvm.EvalFile(file, vars)
	} else {
		jsonString, err = jvm.EvalCode("<inline-code>", vm.MakeCode(code), vars)
	}
	if err != nil {
		return diag.FromErr(err)
	}

	switch format {
	case fmtJSON:
		if err := data.Set(fldComputedResult, jsonString); err != nil {
			return diag.FromErr(err)
		}
	default:
		var d interface{}
		if err := json.Unmarshal([]byte(jsonString), &d); err != nil {
			return diag.FromErr(errors.Wrap(err, "unmarshal data"))
		}
		if format == fmtYAML {
			d = []interface{}{d}
		}
		var out bytes.Buffer
		if err := vmutil.RenderYAMLDocuments(d, &out); err != nil {
			return diag.FromErr(errors.Wrap(err, "render YAML"))
		}
		if err := data.Set(fldComputedResult, out.String()); err != nil {
			return diag.FromErr(err)
		}
	}

	// always run
	data.SetId(strconv.FormatInt(time.Now().Unix(), 10))
	return
}

func getLibPaths(data *schema.ResourceData) ([]string, error) {
	var libPaths []string
	lp0, ok := data.GetOk(fldLibPaths)
	if ok {
		lp1, ok := lp0.([]interface{})
		if !ok {
			return nil, fmt.Errorf("unexpected type for %s, want []interface{} got %v", fldLibPaths, reflect.TypeOf(lp0))
		}
		for _, l := range lp1 {
			libPaths = append(libPaths, fmt.Sprint(l))
		}
	}
	return libPaths, nil
}

func getVariables(data *schema.ResourceData) (res vm.VariableSet, _ error) {
	var extVars []vm.Var
	var tlaVars []vm.Var

	setVars := func(fldName string, fn func(name, value string) vm.Var, tla bool) error {
		vars0, ok := data.GetOk(fldName)
		if !ok {
			return nil
		}
		vars, ok := vars0.(map[string]interface{})
		if !ok {
			return fmt.Errorf("unexpected type for %s, want map[string]interface{} got %v", fldName, reflect.TypeOf(vars0))
		}
		for k, v := range vars {
			val := fmt.Sprint(v)
			userVar := fn(k, val)
			if tla {
				tlaVars = append(tlaVars, userVar)
			} else {
				extVars = append(extVars, userVar)
			}
		}
		return nil
	}

	err := setVars(fldExtVars, vm.NewVar, false)
	if err != nil {
		return res, err
	}
	err = setVars(fldExtCodeVars, vm.NewCodeVar, false)
	if err != nil {
		return res, err
	}

	err = setVars(fldTLAVars, vm.NewVar, true)
	if err != nil {
		return res, err
	}
	err = setVars(fldTLACodeVars, vm.NewCodeVar, true)
	if err != nil {
		return res, err
	}
	return vm.VariableSet{}.WithVars(extVars...).WithTopLevelVars(tlaVars...), nil
}
