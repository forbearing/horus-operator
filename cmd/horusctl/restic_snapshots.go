package horusctl

import (
	"os"

	"github.com/forbearing/horus-operator/pkg/logger"
	"github.com/forbearing/horus-operator/pkg/restic"
	"github.com/forbearing/horus-operator/pkg/types"
	"github.com/forbearing/k8s/util/signals"
	"github.com/spf13/cobra"
)

var (
	storage string
	cluster []string
	tags    []string

	snapshotsCmd = &cobra.Command{
		Use:   "snapshots",
		Short: "List all snapshots",
		Long:  "List all snapshots stored in the repository",
		Run: func(cmd *cobra.Command, args []string) {
			builder.SetLogLevel(logLevel)
			builder.SetLogFormat(logFormat)
			logger.Init()

			restic.Snapshots(signals.NewSignalContext(), types.Storage(storage), cluster, tags, os.Stdout)
		},
	}
)

func init() {
	snapshotsCmd.Flags().StringVarP(&storage, "storage", "s", "", "storage type")
	snapshotsCmd.Flags().StringSliceVarP(&cluster, "cluster", "c", []string{}, "filte restic snapshots by kubernetes cluster name, separated by comma")
	snapshotsCmd.Flags().StringSliceVarP(&tags, "tags", "t", []string{}, "filter restic snapshots by tag name, separated by comma")
	snapshotsCmd.MarkFlagRequired("storage")
	resticCmd.AddCommand(snapshotsCmd)
}
