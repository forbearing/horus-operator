package backup

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	storagev1alpha1 "github.com/forbearing/horus-operator/apis/storage/v1alpha1"
	"github.com/forbearing/horus-operator/pkg/template"
	"github.com/forbearing/horus-operator/pkg/types"
	"github.com/forbearing/horus-operator/pkg/util"
	"github.com/sirupsen/logrus"
)

// createFindpvdirDeployment
func createFindpvdirDeployment(backupObj *storagev1alpha1.Backup, meta pvdataMeta) (string, time.Duration, error) {
	beginTime := time.Now()
	operatorNamespace := util.GetOperatorNamespace()
	podHandler.ResetNamespace(operatorNamespace)
	logger := logrus.WithFields(logrus.Fields{
		"Component":         findpvdirName,
		"OperatorNamespace": operatorNamespace,
	})

	deployName := findpvdirName + "-" + meta.nodeName
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
	execPod, err := createAndGetRunningPod(operatorNamespace, findpvdirBytes)
	if err != nil {
		return "", time.Now().Sub(beginTime), err
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
			return "", time.Now().Sub(beginTime), fmt.Errorf("persistentvolume handler get persistentvolume error: %s", err.Error())
		}
		return pvObj.Spec.HostPath.Path, time.Now().Sub(beginTime), nil
	case types.VolumeLocal:
		pvObj, err := pvHandler.Get(meta.pvname)
		if err != nil {
			return "", time.Now().Sub(beginTime), fmt.Errorf("persistentvolume handler get persistentvolume error: %s", err.Error())
		}
		return pvObj.Spec.Local.Path, time.Now().Sub(beginTime), nil
	}
	logger.Debugf("executing command %v to find persistentvolume data in pod %s", cmdFindpvdir, execPod.GetName())

	// It will execute command "cmdFindpvdir" within pod to find the persistentvolume data directory path
	// and output it to cmdOutput.
	cmdOutput := new(bytes.Buffer)
	if err := podHandler.ExecuteWithStream(execPod.GetName(), "", cmdFindpvdir, os.Stdin, cmdOutput, cmdOutput); err != nil {
		return "", time.Now().Sub(beginTime), fmt.Errorf("%s find the persistentvolume data directory failed: %s", findpvdirName, err.Error())
	}
	podHandler.Execute(execPod.GetName(), "", cmdFindpvdir)
	logger.Debugf("the persistentvolume data path is: %s", strings.TrimSpace(cmdOutput.String()))
	logger.Debugf("the persistentvolume data path is: %s", cmdOutput.String())
	return strings.TrimSpace(cmdOutput.String()), time.Now().Sub(beginTime), nil
}
