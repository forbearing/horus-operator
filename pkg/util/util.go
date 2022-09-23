package util

import (
	"os"

	"github.com/forbearing/horus-operator/pkg/types"
	corev1 "k8s.io/api/core/v1"
)

// GetPodPVPath
func GetPodPVPath(p *corev1.Pod) []string {

	return nil
}

const (
	EnvOperatorNamespace = "OPERATOR_NAMESPACE"
	EnvWatchNamespace    = "WATCH_NAMESPACE"
	EnvCurrentNamespace  = "NAMESPACE"
	namespaceFile        = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
)

// GetCurrentNamespace
func GetCurrentNamespace() string {
	// namespace file exists and is not direcotry
	if fs, err := os.Stat(namespaceFile); err == nil && !fs.IsDir() {
		if data, err := os.ReadFile(namespaceFile); err == nil {
			return string(data)
		}
	}
	return os.Getenv(EnvCurrentNamespace)
}

// GetOperatorNamespace
func GetOperatorNamespace() string {
	var namespace string
	if namespace = os.Getenv(EnvOperatorNamespace); len(namespace) != 0 {
		return namespace
	}
	return types.DefaultOperatorNamespace
}
