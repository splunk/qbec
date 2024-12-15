package commands

import (
	"os"

	"github.com/spf13/cobra"
)

const (
	completionLongDesc = `
Output shell completion code for bash, zsh, and fish.
The shell code must be evaluated to provide interactive
completion of qbec commands. This can be done by sourcing it from
the appropriate shell configuration file.`

	completionExample = `
# Bash completion
# If running bash-completion, write the output code to your bash_completion.d/ directory,
    qbec completion bash > /usr/local/etc/bash_completion.d/qbec

# To load the completion into your current shell
    source <(qbec completion bash)

# Save the completion script to a file and source it in your ~/.bash_profile
    qbec completion bash > ~/qbec.bash
    printf "
    # qbec bash completion
    source '~/qbec.bash'
    " >> ~/.bash_profile
    source ~/.bash_profile

# Zsh completion
# Save the completion script to a file and source it in your ~/.zshrc
    qbec completion zsh > ~/.qbec.zsh
    echo "source ~/.qbec.zsh" >> ~/.zshrc
    source ~/.zshrc

# Fish completion
# Save the completion script to a file and source it in your ~/.config/fish/completions/
    qbec completion fish > ~/.config/fish/completions/qbec.fish
`
)

var (
	// add any custom completion functions for flags here
	customFlagCompletions = map[string]string{}
)

func NewCompletionCommand(root *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "completion [bash|zsh|fish]",
		DisableFlagsInUseLine: true,
		Short:                 "Output shell completion for bash, zsh, or fish",
		Long:                  completionLongDesc,
		Example:               completionExample,
		Args:                  cobra.ExactValidArgs(1),
		ValidArgs:             []string{"bash", "zsh", "fish"},
		RunE: func(cmd *cobra.Command, args []string) error {
			shell := args[0]
			if len(customFlagCompletions) > 0 {
				addCustomFlagCompletions(root)
			}
			switch shell {
			case "bash":
				return root.GenBashCompletion(os.Stdout)
			case "zsh":
				return root.GenZshCompletion(os.Stdout)
			case "fish":
				return root.GenFishCompletion(os.Stdout, true)
			default:
				return cmd.Usage()
			}
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
