package nfs

import (
	"context"
	"testing"

	"github.com/forbearing/k8s/persistentvolume"
	"k8s.io/client-go/tools/clientcmd"
)

func TestNewNFSPV(t *testing.T) {
	handler := persistentvolume.NewOrDie(context.Background(), clientcmd.RecommendedHomeFile)

	pv, err := NewNFSPV(context.Background(), clientcmd.RecommendedHomeFile, "1.1.1.1", "/srv/nfs/kubedata")
	if err != nil {
		t.Fatal(err)
	}

	t.Log(pv.Name)
	handler.Delete(pv)

}
