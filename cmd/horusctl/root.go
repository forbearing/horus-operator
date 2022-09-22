package horusctl

import (
	pkgargs "github.com/forbearing/horus-operator/pkg/args"
	"github.com/forbearing/horus-operator/pkg/logger"
	"github.com/spf13/cobra"
)

var (
	builder   = pkgargs.NewBuilder()
	logLevel  string
	logFormat string
	namespace string

	defaultNamespace = "default"
)

var rootCmd = &cobra.Command{
	Use:   "horusctl",
	Short: "horus-operator command line",
	Long:  "horus-operator command line",
	Run: func(cmd *cobra.Command, args []string) {
		builder.SetLogLevel(logLevel)
		builder.SetLogFormat(logFormat)
		logger.Init()
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "set log level, ('info' or 'debug')")
	rootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "text", "log encoding ('text' or 'json')")
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", defaultNamespace, "the namespace of 'Backup|Restore|Clone|Migration|Traffic' CustomResource")
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}
