package backup

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	storagev1alpha1 "github.com/forbearing/horus-operator/apis/storage/v1alpha1"
	"github.com/forbearing/horus-operator/pkg/template"
	"github.com/forbearing/horus-operator/pkg/types"
	"github.com/forbearing/horus-operator/pkg/util"
	"github.com/pkg/errors"
)

// createFindpvdirDeployment
func createFindpvdirDeployment(backupObj *storagev1alpha1.Backup, meta pvdataMeta) (string, error) {
	beginTime := time.Now().UTC()
	defer func() {
		costedTime = time.Now().UTC().Sub(beginTime)
	}()

	operatorNamespace := util.GetOperatorNamespace()
	podHandler.ResetNamespace(operatorNamespace)

	deployName := findpvdirName + "-" + backupObj.GetName() + "-" + meta.nodeName
	findpvdirBytes := []byte(fmt.Sprintf(
		// the deployment template
		template.FindpvdirDeploymentTemplate,
		// deployment.metadata.name
		// deployment.metadata.namespace
		// deployment name, deployment namespace
		deployName, operatorNamespace,
		// deployment.spec.template.metadata.annotations
		// pod template annotations
		types.AnnotationUpdatedTime, time.Now().Format(time.RFC3339),
		// deployment.spec.template.spec.nodeName
		// deployment.spec.template.spec.containers.image
		// node name, deployment image
		meta.nodeName, findpvdirImage,
		// deployment.spec.template.spec.containers.env
		// the environment variables passed to pods.
		backupObj.Spec.TimeZone))
	execPod, err := filterRunningPod(operatorNamespace, findpvdirBytes)
	if err != nil {
		return "", err
	}

	// if persistentvolume volume source is hostPath or local, the returned value
	// is pvpath not pvdir, and pvpath = pvdir + pvname.
	// And it's no need to find the persistentvolume data directory path now, just return
	// the "hostPath" or "local" in k8s node path.
	cmdFindpvdir := []string{"findpvdir", "--pod-uid", meta.podUID, "--storage-type", meta.volumeSource}
	switch meta.volumeSource {
	case types.VolumeHostPath:
		pvObj, err := pvHandler.Get(meta.pvname)
		if err != nil {
			return "", errors.Wrap(err, "persistentvolume handler get persistentvolume failed")
		}
		return pvObj.Spec.HostPath.Path, nil
	case types.VolumeLocal:
		pvObj, err := pvHandler.Get(meta.pvname)
		if err != nil {
			return "", errors.Wrap(err, "persistentvolume handler get persistentvolume failed")
		}
		return pvObj.Spec.Local.Path, nil
	}
	logger.Debugf("executing command %v to find persistentvolume data in pod %s", cmdFindpvdir, execPod.GetName())

	// It will execute command "cmdFindpvdir" within pod to find the persistentvolume data directory path
	// and output it to cmdOutput.
	cmdOutput := new(bytes.Buffer)
	for i := 1; i <= 12; i++ {
		if err := podHandler.ExecuteWithStream(execPod.GetName(), "", cmdFindpvdir, os.Stdin, cmdOutput, io.Discard); err != nil {
			return "", errors.Wrapf(err, "%s find the persistentvolume data directory failed", findpvdirName)
		}
		if len(strings.TrimSpace(cmdOutput.String())) != 0 {
			break
		}
		logger.Warnf("the persistentvolume data path not found, retry %d", i)
		time.Sleep(time.Second * 5)
	}
	logger.Debugf("the persistentvolume data path is: %s", strings.TrimSpace(cmdOutput.String()))
	return strings.TrimSpace(cmdOutput.String()), nil
}
