package backup

import (
	"fmt"
	"time"

	storagev1alpha1 "github.com/forbearing/horus-operator/apis/storage/v1alpha1"
	"github.com/forbearing/horus-operator/pkg/template"
	"github.com/forbearing/horus-operator/pkg/types"
	"github.com/forbearing/horus-operator/pkg/util"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
)

// createBackup2sftpDeployment create a deployment to backup persistentvolume data to sftp server.
func createBackup2sftpDeployment(backupObj *storagev1alpha1.Backup, meta pvdataMeta) (*corev1.Pod, error) {
	beginTime := time.Now().UTC()
	defer func() {
		costedTime = time.Now().UTC().Sub(beginTime)
	}()

	operatorNamespace := util.GetOperatorNamespace()
	secHandler.ResetNamespace(operatorNamespace)
	secObj, err := secHandler.Get(backupObj.Spec.CredentialName)
	if err != nil {
		return nil, errors.Wrap(err, "secret handler get secret failed")
	}
	//# RESTIC_PASSWORD="restic"; restic -r sftp://horus@10.250.16.21:2222//upload/restic init
	user := string(secObj.Data[envSftpUsername])
	pass := string(secObj.Data[envSftpPassword])
	addr := backupObj.Spec.BackupTo.SFTP.Address
	port := backupObj.Spec.BackupTo.SFTP.Port
	repoPath := backupObj.Spec.BackupTo.SFTP.Path
	resticRepo := fmt.Sprintf("sftp://%s@%s:%d/%s", user, addr, port, repoPath)

	// 1.必须提前创建好 restic repository 目录, 这样以后 restic backup 的时候只需要
	//   输入 sftp server 用户密码一遍就可以了
	// 2.如果没有提前创建好 restic repository 目录, 创建目录的过程也需要输入 sftp 密码,
	//   即需要输入两边密码.
	// 3.但是这里有一个问题, 如果只需要输入一次密码,但是你输入了两次密码, 从 stdin 输出的第二次
	//   密码就会打印到 stdout 来, 这显然不行, 会把密码泄漏了.
	if err := util.MakeDirOnSftp(addr, port, user, pass, repoPath); err != nil {
		return nil, errors.Wrap(err, "mkdir on sftp server failed")
	}

	DeployNameBackup2sftp = theDeployName(backup2sftpName, backupObj, meta)
	credentialName := backupObj.Spec.CredentialName
	backup2sftpBytes := []byte(fmt.Sprintf(
		// the deployment template
		template.TemplateBackup2sftp,
		// deployment.metadata.name
		// deployment.metadata.namespace
		// deployment name, deployment namespace
		DeployNameBackup2sftp, operatorNamespace,
		// deployment.spec.template.metadata.annotations
		// pod template annotations
		types.AnnotationUpdatedTime, time.Now().Format(time.RFC3339),
		// deployment.spec.template.spec.nodeName
		// deployment.spec.template.spec.containers.image
		// node name, deployment image
		meta.nodeName, backup2sftpImage,
		// deployment.spec.template.spec.containers.env
		// the environment variables passed to pods
		backupObj.Spec.TimeZone, types.StorageSFTP, resticRepo,
		credentialName, credentialName, credentialName,
	))
	podObj, err := filterRunningPod(operatorNamespace, backup2sftpBytes)
	if err != nil {
		return nil, err
	}
	return podObj, nil
}
