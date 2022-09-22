package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"
)

var (
	argPodUid      = pflag.String("pod-uid", "", "the pod uid.")
	argStorageType = pflag.String("storage-type", "", "the storage type")
	argKubeletHome = pflag.String("kubelet-home", "/var/lib/kubelet/", "the kubelet home directory path")
)

func main() {
	pflag.Parse()
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	dirname := filepath.Join(*argKubeletHome, "pods", *argPodUid, "volumes")

	dirEntries, err := os.ReadDir(dirname)
	if err != nil {
		return
	}
	for _, file := range dirEntries {
		if file.IsDir() && strings.Contains(file.Name(), *argStorageType) {
			fmt.Println(filepath.Join(dirname, file.Name()))
		}
	}
}
