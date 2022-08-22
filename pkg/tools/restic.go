package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	storagev1alpha "github.com/forbearing/horus-operator/apis/storage/v1alpha1"
	"github.com/forbearing/k8s/deployment"
	"github.com/forbearing/k8s/pod"
	"github.com/forbearing/restic"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	resticBackupSource = "/backup-source"
	resticRepo         = "/resitc-repo"
	resticPasswdFile   = "/tmp/.restic-passwd"
)

var (
	ErrRepoNotSet   = errors.New("restic repository not set")
	ErrPasswdNotSet = errors.New("restic password not set")
	ErrNotFound     = errors.New(`command "restic" not found`)
)

type Restic struct {
	Repo   string
	Passwd string
	Hosts  []string
	Tags   []string

	Cmd string

	// restic command stdout and stderr output.
	stdout io.Writer
	stderr io.Writer

	// set the restic command output format to JSON, default to TEXT.
	JSON bool
}

func (t *Restic) check() error {
	if len(t.Repo) == 0 {
		return ErrRepoNotSet
	}
	if len(t.Passwd) == 0 {
		return ErrPasswdNotSet
	}
	if _, err := exec.LookPath("restic"); err != nil {
		return ErrNotFound
	}

	return nil
}
func (t *Restic) cleanup(ctx context.Context) {
	r, _ := restic.New(ctx, nil)
	r.Command(restic.Unlock{}).Run()
}

// DoBackup start backup data using the restic backup tool.
func (t *Restic) DoBackup(ctx context.Context, object runtime.Object, backupObj *storagev1alpha.Backup) error {
	//storageType := util.GetBackupToStorage(backupObj)
	return nil
}

func (t *Restic) BackupToNFS(ctx context.Context, operatorNamespace string, podObj *corev1.Pod, nfs *storagev1alpha.NFS) (*bytes.Buffer, error) {
	logrus.Info("BackupToNFS")
	handler, err := pod.New(ctx, "", podObj.Namespace)
	if err != nil {
		return nil, err
	}
	res := restic.NewIgnoreNotFound(ctx, &restic.GlobalFlags{NoCache: true, Repo: nfs.Path})
	res.Command(restic.Init{})
	logrus.Info(res.String())

	//_, err = handler.Apply(fmt.Sprintf(findpvpathPod, "findpvpath", "default"))
	//handler.WaitReady(findpvpathPod)
	//if k8serrors.IgnoreAlreadyExists(err); err != nil {
	//    return nil, err
	//}

	//nodeName, _ := handler.GetNodeName(podObj)
	podUID, _ := handler.GetUID(podObj)
	buffer := &bytes.Buffer{}
	handler.ExecuteWithStream("findpvpath", "", []string{"findpvpath", "-uid", podUID, "-storage", "nfs"}, os.Stdin, buffer, buffer)
	return buffer, nil

	//podYaml := generatePodTemplateNFS(podObj.Name, podObj.Namespace, nodeName, strings.Split(buffer.String(), "\n"), nfs.Server, nfs.Path)

	//forbackup, err := handler.Create(podYaml)
	//if err != nil {
	//    return err
	//}
	//return handler.WithNamespace(operatorNamespace).Execute(forbackup.Name, "", strings.Split(res.String(), " "), nil)
}

// DoRestore start restore data using the restic backup tool.
func (t *Restic) DoRestore(ctx context.Context, dst, src string) error {
	defer t.cleanup(ctx)

	return nil
}

// DoMigration start migrate data using the restic backup tool.
func (t *Restic) DoMigration(ctx context.Context, dst, src string) error {
	defer t.cleanup(ctx)

	return nil
}

// DoClone start clone data using the restic backup tool.
func (t *Restic) DoClone(ctx context.Context, dst, src string) error {
	defer t.cleanup(ctx)

	return nil
}

func BackupToNFS(ctx context.Context, operatorNamespace string, podObj *corev1.Pod, nfs *storagev1alpha.NFS) (*bytes.Buffer, error) {
	// === 准备处理器
	deployHandler, err := deployment.New(ctx, "", operatorNamespace)
	if err != nil {
		return nil, err
	}
	podHandler, err := pod.New(ctx, "", operatorNamespace)
	if err != nil {
		return nil, err
	}
	_ = deployHandler
	_ = podHandler

	// === 1.获取 NFS 的信息
	//nfs.Server
	//nfs.Path

	// === 2.创建一个用来查找 pv 路径的 deployment,
	// deployment 需要挂载 /var/lib/kubelet 目录
	// deployment 需要和 operator 部署在同一个 namespace
	// deployment 配置 nodeName 和需要备份的 pod 在同一个 node 上.
	findpvpathDeployName := "findpvpath"
	nodeName, _ := podHandler.GetNodeName(podObj)
	findpvpathImage := "hybfkuf/findpvpath:latest"
	_, err = deployHandler.Apply([]byte(fmt.Sprintf(findpvpathDeploymentTemplate,
		findpvpathDeployName, operatorNamespace, nodeName, findpvpathImage)))
	if err != nil {
		return nil, err
	}
	deployHandler.WaitReady(findpvpathDeployName)

	// === 3.获取查找 pv 路径的 pod
	// 在这个 pod 中执行命令, 来获取需要备份的 pod 的 pv 路径.
	findpvpathPods, err := deployHandler.GetPods(findpvpathDeployName)
	if err != nil {
		return nil, err
	}
	podUID, _ := podHandler.GetUID(podObj)
	buffer := &bytes.Buffer{}
	err = podHandler.ExecuteWithStream(findpvpathPods[0].Name, "", []string{"findpvpath", "-uid", podUID, "-storage", "nfs"}, os.Stdin, buffer, buffer)
	if err != nil {
		return nil, err
	}

	// === 4.创建一个用来备份数据的 deployment,
	// deployment 挂载需要备份的 pod 的 pv,
	// deployment 挂载 NFS 存储
	// 对 deployment 的 pod 执行命令:
	//   restic init 初始化 restic repository
	//   restic backup 将 pv 数据备份到 NFS 存储
	backuptonfsDeployName := "backup-to-nfs"
	backupSource := strings.TrimSpace(buffer.String())
	backuptonfsImage := "hybfkuf/backup-tools-restic:latest"
	_, err = deployHandler.Apply([]byte(fmt.Sprintf(backuptonfsDeploymentTemplate,
		backuptonfsDeployName, operatorNamespace, nodeName, backuptonfsImage, backupSource, nfs.Server, nfs.Path)))
	if err != nil {
		return nil, err
	}
	deployHandler.WaitReady(backuptonfsDeployName)
	backuptonfsPods, err := deployHandler.GetPods(backuptonfsDeployName)
	if err != nil {
		return nil, err
	}

	// === 5.在 pod/backup-to-nfs 中执行 "restic init"
	res := restic.NewIgnoreNotFound(ctx, &restic.GlobalFlags{NoCache: true, Repo: resticRepo, Quiet: true, PasswordFile: resticPasswdFile})
	//res.SetEnv("REPOSITORY_PASSWORD", "mypass")
	logrus.Info(res.Command(restic.Init{}))
	resticPass := &bytes.Buffer{}
	resticPass.WriteString("mypass")
	podHandler.ExecuteWithStream(backuptonfsPods[0].Name, "", strings.Split(res.String(), " "), resticPass, os.Stdout, os.Stderr)

	// === 6.在 pod/backup-to-nfs 中执行 "restic backup"
	logrus.Info(res.Command(restic.Backup{}.SetArgs(resticBackupSource)))
	podHandler.Execute(backuptonfsPods[0].Name, "", strings.Split(res.String(), " "))

	return buffer, nil
}
