package horusctl

import "github.com/spf13/cobra"

var (
	resticCmd = &cobra.Command{
		Use:   "restic",
		Short: "restic command",
		Long:  "restic command",
	}
)

func init() {
	rootCmd.AddCommand(resticCmd)
}
