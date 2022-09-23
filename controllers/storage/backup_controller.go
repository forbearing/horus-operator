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
	"time"

	storagev1alpha1 "github.com/forbearing/horus-operator/apis/storage/v1alpha1"
	"github.com/forbearing/horus-operator/pkg/types"
	"github.com/forbearing/k8s/cronjob"
	"github.com/go-logr/logr"
	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	cjHandler = cronjob.NewOrDie(context.TODO(), "", "")
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
//+kubebuilder:rbac:groups=batchv1,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete

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

	/*
		reqLogger := r.Log.WithValues("Namespace", req.Namespace, "Name", req.Name)
		reqLogger.Info("Reconciling backup controller")

		// 1.get a "Backup" resource
		backupObj := &storagev1alpha1.Backup{}
		err := r.Get(ctx, req.NamespacedName, backupObj)
		if err != nil {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		// Set the default tiemout for backup progress.
		timeout := backupObj.Spec.Timeout.Duration
		if timeout == time.Duration(0) {
			timeout = types.DefaultBackupTimeout
		}
		backupCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		if err := backup.Backup(backupCtx, types.KindBackup, req.Namespace, req.Name); err != nil {
			return ctrl.Result{}, err
		}
	*/

	logger := r.Log.WithValues("Namespace", req.Namespace, "Name", req.Name)

	// Get backup object and ignore "NotFound" error.
	backupObj := &storagev1alpha1.Backup{}
	if err := r.Get(ctx, req.NamespacedName, backupObj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// ====================
	// handle cronjob
	// ====================

	// Construct a cronjob object
	cronjobObject := r.cronjobForBackup(backupObj)
	cj := &batchv1.CronJob{}
	// get the cronjob resource
	namespacedName := apitypes.NamespacedName{Namespace: req.NamespacedName.Namespace, Name: "backup" + "-" + req.NamespacedName.Name}
	if err := r.Get(ctx, namespacedName, cj); err != nil {
		// if cronjob resource not exits, create it.
		if apierrors.IsNotFound(err) {
			if err := r.Create(ctx, cronjobObject); err != nil {
				logger.Error(err, "create cronjob failed")
				return ctrl.Result{}, err
			}
			logger.Info("Successfully create cronjob/" + cronjobObject.GetName())
		} else {
			// get cronjob error
			logger.Error(err, "get cronjob failed")
			return ctrl.Result{}, err
		}
	} else {
		// if cronjob resource already exist, update it.
		if err := r.Update(ctx, cronjobObject); err != nil {
			logger.Error(err, "update cronjob failed")
			return ctrl.Result{}, err
		}
		logger.Info("Successfully udpate cronjob/" + cronjobObject.GetName())
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&storagev1alpha1.Backup{}).
		Owns(&batchv1.CronJob{}).
		Complete(r)
}

var (
	LabelName       = "app.kubernetes.io/name"
	LabelRole       = "app.kubernetes.io/role"
	LabelBackupTool = "app.kubernetes.io/backup-tool"
	LabelPartOf     = "app.kubernetes.io/part-of"
)

// cronjobForBackup construct a *batch1.CronJob resource with the same namespace
// and name as *storagev1alpha1.Backup.
func (r *BackupReconciler) cronjobForBackup(b *storagev1alpha1.Backup) *batchv1.CronJob {
	labels := make(map[string]string)
	annotations := make(map[string]string)
	for k, v := range b.Labels {
		labels[k] = v
	}
	labels[LabelName] = "horusctl"
	labels[LabelRole] = "backup"
	labels[LabelBackupTool] = "restic"
	labels[LabelPartOf] = types.DefaultOperatorName
	for k, v := range b.Annotations {
		annotations[k] = v
	}

	successJobLimit := new(int32)
	failedJobLimit := new(int32)
	*successJobLimit, *failedJobLimit = 3, 3
	cronjob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "backup" + "-" + b.Name,
			Namespace:   b.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: batchv1.CronJobSpec{
			Schedule:                   b.Spec.Schedule,
			ConcurrencyPolicy:          batchv1.ForbidConcurrent,
			SuccessfulJobsHistoryLimit: successJobLimit,
			FailedJobsHistoryLimit:     successJobLimit,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: labels,
						},
						Spec: corev1.PodSpec{
							RestartPolicy:      corev1.RestartPolicyNever,
							ServiceAccountName: "horusctl",
							Containers: []corev1.Container{
								{
									Name:  "horusctl",
									Image: "hybfkuf/horusctl:latest",
									Command: []string{
										"horusctl",
										"backup",
										"--namespace", b.GetNamespace(), b.Spec.BackupFrom.Name,
									},
									Env: []corev1.EnvVar{
										{
											Name:  "TZ",
											Value: b.Spec.TimeZone,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	cronjob.Annotations[types.AnnotationCreatedTime] = time.Now().Format(time.RFC3339)
	ctrl.SetControllerReference(b, cronjob, r.Scheme)

	return cronjob
}

// handleFinalizer add finalizer when create/update Backup object, and remove
// finalizer when delete Backup Object
func (r *BackupReconciler) handleFinalizer(ctx context.Context, backupObj *storagev1alpha1.Backup) error {
	// name of our custom finalizer
	finalizerName := types.DefaultBackupFinalizerName
	// examine DeletionTimestamp to determine if object is under deletion
	if backupObj.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(backupObj, finalizerName) {
			ok := controllerutil.AddFinalizer(backupObj, finalizerName)
			logrus.Infof("Add Finalizer %s : %t", finalizerName, ok)
			return r.Update(ctx, backupObj)
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(backupObj, finalizerName) {
			// our finalizer is present, so lets handle any external dependency
			if err := r.deleteExternalResources(backupObj); err != nil {
				// if fail to delete the external dependency here, return with error
				// so that it can be retried
				return err
			}
			// remove our finalizer from the list and update it.
			ok := controllerutil.RemoveFinalizer(backupObj, finalizerName)
			logrus.Infof("Remove Finalizer %s : %t", finalizerName, ok)
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
