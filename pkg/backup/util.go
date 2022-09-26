package backup

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"time"

	storagev1alpha1 "github.com/forbearing/horus-operator/apis/storage/v1alpha1"
	"github.com/forbearing/horus-operator/pkg/types"
	"github.com/forbearing/k8s/deployment"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// filterRunningPod creates the deployment and get its any running status pod.
// The namespace determine which namespace the deployment object deploy to.
func filterRunningPod(namespace string, deployData interface{}) (*corev1.Pod, error) {
	depHandler.ResetNamespace(namespace)
	rsHandler.ResetNamespace(namespace)
	podHandler.ResetNamespace(namespace)

	// 1.apply deployment
	deployObj, err := depHandler.Apply(deployData)
	if err != nil {
		return nil, fmt.Errorf("deployment handler Apply error: %s", err.Error())
	}
	// 2.block here and wait the deployment to be available and ready.
	if err := depHandler.WaitReady(deployObj.GetName()); err != nil {
		return nil, fmt.Errorf("deployment handler WaitReady %s error: %s", deployObj.GetName(), err.Error())
	}
	// 3.get all pods object owned by the deployment.
	podObjs, err := depHandler.GetPods(deployObj)
	if err != nil {
		return nil, fmt.Errorf("replicaset handler get %s all replicasets error: %s", deployObj.GetName(), err.Error())
	}
	// 4.if the DeletionTimestamp of pod is zero/nil and return it.
	//   running pods doesn't have DeletionTimestamp field.
	//   DeletionTimestamp field only set in Terminating status pods.
	for _, podObj := range podObjs {
		// if DeletionTimestamp not zero/nil, it means that the pod is Terminating.
		if !podObj.DeletionTimestamp.IsZero() {
			continue
		}
		// if pod.Status.Phase not PodRunning, such as Pending, it means that the pod is "ContainerCreating",
		// it's not necessary to check the pod status.phase after call "WaitReady" method.
		if podObj.Status.Phase != corev1.PodRunning {
			continue
		}
		return podObj, nil
	}
	return nil, fmt.Errorf("not found running pod for deployment/%s", deployObj.GetName())
}

// filterRunningPod2 creates the deployment and get its any running status pod.
// The namespace determine which namespace the deployment object deploy to.
func filterRunningPod2(namespace string, deployData interface{}) (*corev1.Pod, error) {
	depHandler.ResetNamespace(namespace)
	rsHandler.ResetNamespace(namespace)
	podHandler.ResetNamespace(namespace)

	// 1.apply deployment
	deployObj, err := depHandler.Apply(deployData)
	if err != nil {
		return nil, fmt.Errorf("deployment handler Apply error: %s", err.Error())
	}

	// 2.block here and wait the deployment to be available and ready.
	if err := depHandler.WaitReady(deployObj.GetName()); err != nil {
		return nil, fmt.Errorf("deployment handler WaitReady %s error: %s", deployObj.GetName(), err.Error())
	}

	// 3.get all replicasets object owned by the deployment.
	rsObjList, err := depHandler.GetRS(deployObj)
	if err != nil {
		return nil, fmt.Errorf("replicaset handler get %s all replicasets error: %s", deployObj.GetName(), err.Error())
	}

	var podsObj []*corev1.Pod
	for i := range rsObjList {
		// 4.find the current working replicaset.
		// the replicaset that .spec.replicas not equal nil and greater than zero always
		// is the working replicaset.
		rsObj := rsObjList[i]
		if rsObj.Spec.Replicas != nil && *rsObj.Spec.Replicas > 0 {
			// 5.get all pods object owned by the replicaset.
			if podsObj, err = rsHandler.GetPods(rsObj); err != nil {
				return nil, fmt.Errorf("pod handler get %s all pods error: %s", rsObj.GetName(), err.Error())
			}
			break
		}
	}

	// 6.if any pods object is running status, return one of them.
	var podObj *corev1.Pod
	for i := range podsObj {
		podObj = podsObj[i]
		if podObj.Status.Phase == corev1.PodRunning {
			return podObj, nil
		}
	}

	return nil, fmt.Errorf("not found running pod for deployment/%s", deployObj.GetName())
}

// parseStorage parse the backup.spec.backupTo field to know where we should backup to
func parseStorage(backupObj *storagev1alpha1.Backup) []types.Storage {
	t := reflect.TypeOf(backupObj.Spec.BackupTo).Elem()
	v := reflect.ValueOf(backupObj.Spec.BackupTo).Elem()

	var storages []types.Storage
	for i := 0; i < v.NumField(); i++ {
		val := v.Field(i).Interface()
		if !reflect.ValueOf(val).IsNil() {
			tag := t.Field(i).Tag.Get("json")
			storage := types.Storage(strings.Split(tag, ",")[0])
			storages = append(storages, storage)
		}
	}
	return storages
}

// createPassStdin 创建一个 *bytes.Buffer 对象, 该对象包含了 restic 密码
// 之所以额外使用这个函数,是因为输入密码之后要会车,并且有些时候需要输入两边密码
func createPassStdin(pass string, repeatCount ...uint) *bytes.Buffer {
	buf := new(bytes.Buffer)
	if len(repeatCount) == 0 {
		buf.WriteString(pass + "\n")
		return buf
	}
	for i := uint(0); i < repeatCount[0]; i++ {
		buf.WriteString((pass + "\n"))
	}
	return buf
}

// setPodTemplateAnnotations 目的就是为了 rollout restart deployment
// 添加一个 annotations 让 pod 重启, 改变 deployment 的 annotations 并不会让 pod 重启,
// 需要修改 deployment 的 spec.template.spec.annotations 字段
func setPodTemplateAnnotations(deploy *appsv1.Deployment) *appsv1.Deployment {
	podAnnotations := deploy.Spec.Template.Annotations
	if podAnnotations == nil {
		podAnnotations = make(map[string]string)
	}
	podAnnotations[types.AnnotationRestartedTime] = time.Now().Format(time.RFC3339)
	deploy.Spec.Template.Annotations = podAnnotations
	return deploy
}

// getOrApplyDeployment 取 deployment, 如果获取不到就是 apply 一个 deployment
func getOrApplyDeployment(handler *deployment.Handler, data []byte) (*appsv1.Deployment, error) {
	deploy, err := handler.Get(data)
	if err == nil {
		return deploy, nil
	}
	return handler.Apply(data)
}
