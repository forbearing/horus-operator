package tools

import (
	"bytes"
	"time"

	"github.com/forbearing/k8s/deployment"
	appsv1 "k8s.io/api/apps/v1"
)

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
	podAnnotations[restartedTimeAnnotation] = time.Now().Format(time.RFC3339)
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
