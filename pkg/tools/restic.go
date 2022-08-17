package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"

	storagev1alpha "github.com/forbearing/horus-operator/apis/storage/v1alpha1"
	"github.com/forbearing/k8s/deployment"
	"github.com/forbearing/k8s/pod"
	"github.com/forbearing/restic"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
	handler.ExecuteWriter("findpvpath", "", []string{"findpvpath", "-uid", podUID, "-storage", "nfs"}, buffer, buffer)
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

//func findPVPath(podUID, storageType string) ([]string, error) {
//    prefix := "/var/lib/kubelet/pods"
//    dirname := filepath.Join(prefix, podUID, "volumes")
//    files, err := ioutil.ReadDir(dirname)
//    if err != nil {
//        return nil, err
//    }

//    var dataPath []string
//    for _, f := range files {
//        if f.IsDir() && strings.Contains(filepath.Join(dirname, f.Name()), strings.ToLower(storageType)) {
//            dataPath = append(dataPath, filepath.Join(dirname, f.Name()))
//        }
//    }
//    return dataPath, nil
//}

func generatePodTemplateNFS(name, namespace, nodeName string, dataPath []string, nfsServer, nfsPath string) string {
	return ""
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
	res := restic.NewIgnoreNotFound(ctx, &restic.GlobalFlags{NoCache: true, Repo: nfs.Path})
	res.Command(restic.Init{})
	logrus.Info(res.String())

	// === 1.获取 NFS 的信息
	//nfs.Server
	//nfs.Path

	// === 2.创建一个 deployment,
	// deployment 需要挂载 /var/lib/kubelet 目录
	// deployment 部署在 operator 同一个 namespace
	// deployment 配置 nodeName 和需要备份的 pod 在同一个 node 上.
	deployName := "findpvpath"
	nodeName, _ := podHandler.GetNodeName(podObj)
	_, err = deployHandler.Apply([]byte(fmt.Sprintf(findpvpathDeployTemplate, deployName, operatorNamespace, nodeName)))
	if err != nil {
		log.Fatal(err)
	}
	deployHandler.WaitReady(deployName)

	// === 3.获取这个 deployment 的 pod
	// 在这个 pod 中执行命令, 来获取需要备份的 pod 的 pv 路径.
	findpvpathPods, err := deployHandler.GetPods(deployName)
	if err != nil {
		return nil, err
	}
	podUID, _ := podHandler.GetUID(podObj)
	buffer := &bytes.Buffer{}
	err = podHandler.ExecuteWriter(findpvpathPods[0].Name, "", []string{"findpvpath", "-uid", podUID, "-storage", "nfs"}, buffer, buffer)
	if err != nil {
		return nil, err
	}
	return buffer, nil

	// === 4.再创建一个 deployment, deployment 挂载需要备份的 pod 的 pv, 再挂载 NFS.
	//   最后把 pv 数据备份进入到 NFS 中.

	//podYaml := generatePodTemplateNFS(podObj.Name, podObj.Namespace, nodeName, strings.Split(buffer.String(), "\n"), nfs.Server, nfs.Path)

	//forbackup, err := handler.Create(podYaml)
	//if err != nil {
	//    return err
	//}
	//return handler.WithNamespace(operatorNamespace).Execute(forbackup.Name, "", strings.Split(res.String(), " "), nil)
}
