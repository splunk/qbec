// Copyright 2021 Splunk Inc.
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

package commands

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/cmd"
)

const (
	completionLongDesc = `
Output shell completion code for bash.
The shell code must be evaluated to provide interactive
completion of qbec commands.  This can be done by sourcing it from
the .bash_profile.`

	completionExample = `
# If running bash-completion, write the output code to your
bash_completion.d/ directory,
	qbec completion > /usr/local/etc/bash_completion.d/qbec

# To load the completion into your current shell
	source <(qbec completion)

# Save the completion script to a file and source it in your ~/.bash_profile
	qbec completion > ~/qbec.bash
	printf "
	# qbec bash completion
	source '~/qbec.bash'
	" >> ~/.bash_profile
	source ~/.bash_profile
`
	// BashCompletionFunc contains all the custom bash functions that are
	// used to generate dynamic completion lists to extend the completion
	// capabilities
	BashCompletionFunc = `
__qbec_override_root() {
    local root pair
    for w in "${words[@]}"; do
    	if [ -n "${pair}" ]; then
	    root="--root=${w}"
	fi
        case "${w}" in
            --root=*)
		root="${w}"
                ;;
            --root)
	    	pair=1
                ;;
        esac
    done
    if [ -n "${root}" ]; then
    	echo -n "${root} "
    fi
}

__qbec_get_envs() {
    if env_list=$(qbec $(__qbec_override_root) env list 2>/dev/null); then
    	COMPREPLY+=( $( compgen -W "${env_list[*]}" -- "$cur" ))
    fi
}

__qbec_custom_func() {
    case ${last_command} in
        qbec_apply | qbec_component_diff | qbec_component_list | qbec_delete |\
	qbec_diff | qbec_env_vars | qbec_param_diff | qbec_param_list |\
	qbec_show | qbec_validate)
	    __qbec_get_envs
	    return
	    ;;
	qbec_completion | qbec_component | qbec_env_list | qbec_env | qbec_init |\
	qbec_param | qbec_version)
	    return
	    ;;
	*)
	    ;;
    esac
}

# remove ':' as a word break since it is used inside flags
COMP_WORDBREAKS=${COMP_WORDBREAKS//:}
`
)

var (
	// add any custom completion functions for flags here
	customFlagCompletions = map[string]string{}
)

func newCompletionCommand(root *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "completion",
		DisableFlagsInUseLine: true,
		Short:                 "Output shell completion for bash",
		Long:                  completionLongDesc,
		Example:               completionExample,
		RunE: func(_ *cobra.Command, _ []string) error {
			if len(customFlagCompletions) > 0 {
				addCustomFlagCompletions(root)
			}
			return cmd.WrapError(root.GenBashCompletion(os.Stdout))
		},
	}
	return cmd
}

func addCustomFlagCompletions(c *cobra.Command) {
	for name, completion := range customFlagCompletions {
		if f := c.Flags().Lookup(name); f != nil {
			if f.Annotations == nil {
				f.Annotations = map[string][]string{}
			}
			f.Annotations[cobra.BashCompCustom] = append(f.Annotations[cobra.BashCompCustom], completion)
		}
	}

	// recursively go through every command
	for _, cmd := range c.Commands() {
		addCustomFlagCompletions(cmd)
	}
}
