package args

var horusHolder = &holder{}

type holder struct {
	kubeconfig string
	logLevel   string
	logFormat  string
	logFile    string
}

func GetKubeconfig() string { return horusHolder.kubeconfig }
func GetLogLevel() string   { return horusHolder.logLevel }
func GetLogFormat() string  { return horusHolder.logFormat }
func GetLogFile() string    { return horusHolder.logFile }
