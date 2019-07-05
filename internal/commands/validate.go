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

package commands

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/remote/k8smeta"
)

const (
	escGreen = "\x1b[32m"
	escRed   = "\x1b[31m"
	escDim   = "\x1b[2m"
	escReset = "\x1b[0m"

	unicodeCheck    = "\u2714"
	unicodeX        = "\u2718"
	unicodeQuestion = "\u003f"
)

type validatorStats struct {
	l          sync.Mutex
	ValidCount int      `json:"valid,omitempty"`
	Unknown    []string `json:"unknown,omitempty"`
	Invalid    []string `json:"invalid,omitempty"`
	Errors     []string `json:"errors,omitempty"`
}

func (v *validatorStats) valid(s string) {
	v.l.Lock()
	defer v.l.Unlock()
	v.ValidCount++
}

func (v *validatorStats) invalid(s string) {
	v.l.Lock()
	defer v.l.Unlock()
	v.Invalid = append(v.Invalid, s)
}

func (v *validatorStats) unknown(s string) {
	v.l.Lock()
	defer v.l.Unlock()
	v.Unknown = append(v.Unknown, s)
}

func (v *validatorStats) errors(s string) {
	v.l.Lock()
	defer v.l.Unlock()
	v.Errors = append(v.Errors, s)
}

type validator struct {
	w                      io.Writer
	client                 Client
	stats                  validatorStats
	red, green, dim, reset string
	silent                 bool
}

func (v *validator) validate(obj model.K8sLocalObject) error {
	name := v.client.DisplayName(obj)
	schema, err := v.client.ValidatorFor(obj.GroupVersionKind())
	if err != nil {
		if err == k8smeta.ErrSchemaNotFound {
			if !v.silent {
				fmt.Fprintf(v.w, "%s%s %s: no schema found, cannot validate%s\n", v.dim, unicodeQuestion, name, v.reset)
			}
			v.stats.unknown(name)
			return nil
		}
		fmt.Fprintf(v.w, "%s%s %s: schema fetch error %v%s\n", v.red, unicodeX, name, err, v.reset)
		v.stats.errors(name)
		return err
	}
	errs := schema.Validate(obj.ToUnstructured())
	if len(errs) == 0 {
		if !v.silent {
			fmt.Fprintf(v.w, "%s%s %s is valid%s\n", v.green, unicodeCheck, name, v.reset)
		}
		v.stats.valid(name)
		return nil
	}
	var lines []string
	for _, e := range errs {
		lines = append(lines, e.Error())
	}
	fmt.Fprintf(v.w, "%s%s %s is invalid\n\t- %s%s\n", v.red, unicodeX, name, strings.Join(lines, "\n\t- "), v.reset)
	v.stats.invalid(name)
	return nil
}

func validateObjects(objs []model.K8sLocalObject, client Client, parallel int, colors bool, out io.Writer, silent bool) error {
	v := &validator{
		w:      &lockWriter{Writer: out},
		client: client,
		silent: silent,
	}
	if colors {
		v.green = escGreen
		v.red = escRed
		v.dim = escDim
		v.reset = escReset
	}

	vErr := runInParallel(objs, v.validate, parallel)
	printStats(v.w, &v.stats)

	switch {
	case vErr != nil:
		return vErr
	case len(v.stats.Invalid) > 0:
		return fmt.Errorf("%d invalid objects found", len(v.stats.Invalid))
	default:
		return nil
	}
}

type validateCommandConfig struct {
	*Config
	parallel   int
	silent     bool
	filterFunc func() (filterParams, error)
}

func doValidate(args []string, config validateCommandConfig) error {
	if len(args) != 1 {
		return newUsageError("exactly one environment required")
	}
	env := args[0]
	if env == model.Baseline {
		return newUsageError("cannot validate baseline environment, use a real environment")
	}
	fp, err := config.filterFunc()
	if err != nil {
		return err
	}
	client, err := config.Client(env)
	if err != nil {
		return err
	}
	objects, err := filteredObjects(config.Config, env, client.ObjectKey, fp)
	if err != nil {
		return err
	}
	return validateObjects(objects, client, config.parallel, config.Colorize(), config.Stdout(), config.silent)

}

func newValidateCommand(cp ConfigProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "validate <environment>",
		Short:   "validate one or more components against the spec of a kubernetes cluster",
		Example: validateExamples(),
	}

	config := validateCommandConfig{
		filterFunc: addFilterParams(cmd, true),
	}

	cmd.Flags().IntVar(&config.parallel, "parallel", 5, "number of parallel routines to run")
	cmd.Flags().BoolVar(&config.silent, "silent", false, "do not print success messages for every object")
	cmd.RunE = func(c *cobra.Command, args []string) error {
		config.Config = cp()
		return wrapError(doValidate(args, config))
	}
	return cmd
}
