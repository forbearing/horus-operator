package util

import (
	"context"

	"github.com/forbearing/k8s/job"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func JobWithCommand(ctx context.Context,
	kubeconfig, namespace, jobName string, command []string) (*batchv1.Job, error) {
	handler, err := job.New(ctx, kubeconfig, namespace)
	if err != nil {
		return nil, err
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:    jobName,
							Image:   "hybfkuf/backup-tools-restic:latest",
							Command: command,
						},
					},
				},
			},
		},
	}

	handler.Delete(job)
	return handler.Create(job)
}

func JobWithScript(ctx context.Context, jobName string, script string) *batchv1.Job {

	return nil
}
