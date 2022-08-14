package nfs

import (
	"context"
	"errors"
	"fmt"
	"os/exec"

	"github.com/forbearing/k8s/persistentvolume"
	corev1 "k8s.io/api/core/v1"
)

var (
	ErrNFSServerNotSet = errors.New("nfs server not set")
	ErrNFSPathNotSet   = errors.New("nfs path not set")
	ErrMountNotFound   = errors.New(`command "moount"not found`)
	ErrUMountNotFound  = errors.New(`command "umount" not found`)
)

// Mount will mont nfs storage to local.
func Mount(ctx context.Context, server, path string) error {
	if len(server) == 0 {
		return ErrNFSServerNotSet
	}
	if len(path) == 0 {
		return ErrNFSPathNotSet
	}
	if _, err := exec.LookPath("mount"); err != nil {
		return ErrMountNotFound
	}

	return nil
}

// Unmont will unmount nfs storage.
func Unmont(ctx context.Context, path string) error {
	if len(path) == 0 {
		return ErrNFSPathNotSet
	}
	if _, err := exec.LookPath("umount"); err != nil {
		return ErrUMountNotFound
	}

	return nil
}

// NewNFSPV create a persistentvolume.
func NewNFSPV(ctx context.Context, kubeconfig, server, path string) (*corev1.PersistentVolume, error) {
	if len(server) == 0 {
		return nil, ErrNFSServerNotSet
	}
	if len(path) == 0 {
		return nil, ErrNFSPathNotSet
	}

	pvYaml := string(fmt.Sprintf(`
apiVersion: v1
kind: PersistentVolume
metadata:
  name: %s
spec:
  capacity:
    storage: %s
  accessModes: ["ReadWriteOnce", "ReadWriteMany", "ReadOnlyMany"]
  persistentVolumeReclaimPolicy: Retain
  nfs:
    server: %s
    path: %s
`, "pv-nfs", "8Gi", server, path))

	handler, err := persistentvolume.New(ctx, kubeconfig)
	if err != nil {
		return nil, err
	}
	return handler.Create([]byte(pvYaml))
}
