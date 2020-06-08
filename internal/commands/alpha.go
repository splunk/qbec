package commands

import "github.com/spf13/cobra"

func newAlphaCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "alpha",
		Short: "experimental qbec commands",
	}
	return cmd
}
