package horusctl

import (
	"github.com/forbearing/horus-operator/pkg/backup"
	"github.com/forbearing/horus-operator/pkg/logger"
	"github.com/forbearing/k8s/util/signals"
	"github.com/spf13/cobra"
)

var (
	backupCmd = &cobra.Command{
		Use:   "backup",
		Short: "backup k8s resource",
		Long:  "backup k8s deployment/statefulset/daemonset/pod",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			builder.SetLogLevel(logLevel)
			builder.SetLogFormat(logFormat)
			logger.Init()

			for _, arg := range args {
				backup.Do(signals.NewSignalContext(), namespace, arg)
			}
		},
	}
)

func init() {
	rootCmd.AddCommand(backupCmd)
}
