/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package storage

import (
	"context"
	"os"
	"time"

	storagev1alpha1 "github.com/forbearing/horus-operator/apis/storage/v1alpha1"
	"github.com/forbearing/horus-operator/pkg/tools"
	"github.com/go-logr/logr"
	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	defaultFinalizerName     = "backup.hybfkuf.io/finalizer"
	defaultOperatorNamespace = "horus-operator"
	defaultOperatorName      = "horus-operator"
	defaultTimeout           = time.Minute * 10
)

// BackupReconciler reconciles a Backup object
type BackupReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=storage.hybfkuf.io,resources=backups,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=storage.hybfkuf.io,resources=backups/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=storage.hybfkuf.io,resources=backups/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Backup object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *BackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	//logger := log.FromContext(ctx)
	//_ = logger
	//logger := logrus.WithFields(logrus.Fields{
	//    "Component": defaultOperatorName,
	//})
	reqLogger := r.Log.WithValues("Namespace", req.Namespace, "Name", req.Name)
	reqLogger.Info("Reconciling backup controller")

	namespace := os.Getenv("NAMESPACE")
	if len(namespace) == 0 {
		namespace = defaultOperatorNamespace
	}

	// 1.get a "Backup" resource
	backupObj := &storagev1alpha1.Backup{}
	err := r.Get(ctx, req.NamespacedName, backupObj)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Set the default tiemout for backup progress.
	timeout := backupObj.Spec.Timeout.Duration
	if timeout == time.Duration(0) {
		timeout = defaultTimeout
	}
	backupCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := tools.BackupToNFS(backupCtx, namespace,
		backupObj, backupObj.Spec.BackupTo.NFS); err != nil {
		return ctrl.Result{}, err
	}
	// =====

	//// 2.get cronjob resource.
	//cronjob := &batchv1.CronJob{}
	//err = r.Get(ctx, types.NamespacedName{Name: req.NamespacedName.Name, Namespace: req.NamespacedName.Namespace}, cronjob)
	//if err != nil {
	//    if apierrors.IsNotFound(err) {
	//        if err = r.Create(ctx, r.cronjobForBackup(backupObj)); err != nil {
	//            // create cronjob failed, return with error.
	//            return ctrl.Result{}, err
	//        }
	//        // create cronjob success and return nil, reconcile again to
	//        // make sure the "backup" resource status met desired status.
	//        return ctrl.Result{Requeue: true}, nil
	//    } else {
	//        // get cronjob failed and not "NotFound" error, return with error.
	//        return ctrl.Result{}, err
	//    }
	//}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&storagev1alpha1.Backup{}).
		//Owns(&batchv1.CronJob{}).
		Complete(r)
}

// cronjobForBackup construct a *batch1.CronJob resource with the same namespace
// and name as *storagev1alpha1.Backup.
func (r *BackupReconciler) cronjobForBackup(b *storagev1alpha1.Backup) *batchv1.CronJob {
	labels := make(map[string]string)
	annotations := make(map[string]string)
	for k, v := range b.Labels {
		labels[k] = v
	}
	for k, v := range b.Annotations {
		annotations[k] = v
	}

	//jobName := fmt.Sprintf("%s-%s", b.Name, time.Now().Unix())
	//job := &batchv1.CronJob{
	//    ObjectMeta: metav1.ObjectMeta{
	//        Name:        jobName,
	//        Namespace:   b.Namespace,
	//        Labels:      labels,
	//        Annotations: annotations,
	//    },
	//}
	//job.Annotations["backup.hybfkuf.io/scheduled-at"] = time.Now().Format(time.RFC3339)

	cronjob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:        b.Name,
			Namespace:   b.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: batchv1.CronJobSpec{
			Schedule:          b.Spec.Schedule,
			ConcurrencyPolicy: batchv1.ForbidConcurrent,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyNever,
							Containers: []corev1.Container{
								{
									Name:    "backup-restic",
									Image:   "hybfkuf/backup-tools-restic:latest",
									Command: []string{"restic", "version"},
								},
							},
						},
					},
				},
			},
		},
	}
	cronjob.Annotations["backup.hybfkuf.io/created-at"] = time.Now().Format(time.RFC3339)
	ctrl.SetControllerReference(b, cronjob, r.Scheme)

	return cronjob
}

// handleFinalizer add finalizer when create/update Backup object, and remove
// finalizer when delete Backup Object
func (r *BackupReconciler) handleFinalizer(ctx context.Context, backupObj *storagev1alpha1.Backup) error {
	// name of our custom finalizer
	myFinalizerName := defaultFinalizerName
	// examine DeletionTimestamp to determine if object is under deletion
	if backupObj.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(backupObj, myFinalizerName) {
			ok := controllerutil.AddFinalizer(backupObj, myFinalizerName)
			logrus.Infof("Add Finalizer %s : %t", myFinalizerName, ok)
			return r.Update(ctx, backupObj)
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(backupObj, myFinalizerName) {
			// our finalizer is present, so lets handle any external dependency
			if err := r.deleteExternalResources(backupObj); err != nil {
				// if fail to delete the external dependency here, return with error
				// so that it can be retried
				return err
			}
			// remove our finalizer from the list and update it.
			ok := controllerutil.RemoveFinalizer(backupObj, myFinalizerName)
			logrus.Infof("Remove Finalizer %s : %t", myFinalizerName, ok)
			return r.Update(ctx, backupObj)
		}
		// Stop reconciliation as the item is being deleted
		return nil
	}
	// Stop reconciliation as the item is being deleted
	return nil
}

// deleteExternalResources
func (r *BackupReconciler) deleteExternalResources(backupObj *storagev1alpha1.Backup) error {
	//
	// delete any external resources associated with the cronJob
	//
	// Ensure that delete implementation is idempotent and safe to invoke
	// multiple times for same object.

	return nil
}
