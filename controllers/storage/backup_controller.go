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
	"fmt"
	"os"

	storagev1alpha1 "github.com/forbearing/horus-operator/apis/storage/v1alpha1"
	"github.com/forbearing/horus-operator/controllers/common"
	"github.com/forbearing/horus-operator/pkg/template"
	"github.com/forbearing/horus-operator/pkg/types"
	"github.com/forbearing/horus-operator/pkg/util"
	"github.com/forbearing/k8s/clusterrolebinding"
	"github.com/forbearing/k8s/cronjob"
	"github.com/forbearing/k8s/namespace"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

/*
TODO:
1.IgnoreAlreadyExist when create serviceaccount
2.IgnoreAlreadyExist error when create cronjob
*/
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
//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete

//// RBAC Management
////+kubebuilder:rbac:groups=authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
////+kubebuilder:rbac:groups=authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
////+kubebuilder:rbac:groups=authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
////+kubebuilder:rbac:groups=authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
////+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
////+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
////+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
////+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
////+kubebuilder:rbac:groups=roles.rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
////+kubebuilder:rbac:groups=roles.rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
////+kubebuilder:rbac:groups=roles.rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
////+kubebuilder:rbac:groups=roles.rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete

// RBAC Management
//+kubebuilder:rbac:groups=authorization.k8s.io,resources=roles,verbs=*
//+kubebuilder:rbac:groups=authorization.k8s.io,resources=rolebindings,verbs=*
//+kubebuilder:rbac:groups=authorization.k8s.io,resources=clusterroles,verbs=*
//+kubebuilder:rbac:groups=authorization.k8s.io,resources=clusterrolebindings,verbs=*
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=*
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=*
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=*
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=*
//+kubebuilder:rbac:groups=roles.rbac.authorization.k8s.io,resources=roles,verbs=*
//+kubebuilder:rbac:groups=roles.rbac.authorization.k8s.io,resources=rolebindings,verbs=*
//+kubebuilder:rbac:groups=roles.rbac.authorization.k8s.io,resources=clusterroles,verbs=*
//+kubebuilder:rbac:groups=roles.rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=*

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
	logger := r.Log.WithValues("namespace", req.Namespace, "name", req.Name)

	// Get backup object and ignore "NotFound" error.
	backupObj := &storagev1alpha1.Backup{}
	if err := r.Get(ctx, req.NamespacedName, backupObj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// =========================
	// reconcile ServiceAccount
	// =========================
	// Construct a serviceaccount object.
	serviceAccount := r.serviceAccountForBackup(backupObj)
	//r.withNamespace(ctx, serviceAccount, types.DefaultBackupJobNamespace)
	//namespacedName := apitypes.NamespacedName{Namespace: types.DefaultBackupJobNamespace, Name: types.DefaultServiceAccountName}
	namespacedName := apitypes.NamespacedName{Namespace: req.NamespacedName.Namespace, Name: "horusctl"}
	// get the serviceaccount resource.
	if err := r.Get(ctx, namespacedName, &corev1.ServiceAccount{}); err != nil {
		if apierrors.IsNotFound(err) {
			if err := r.Create(ctx, serviceAccount); err != nil {
				logger.Error(err, "create serviceaccount failed")
				return ctrl.Result{}, err
			}
			logger.Info("Successfully create serviceaccount/" + serviceAccount.GetName())
			return ctrl.Result{Requeue: true}, nil
		} else {
			logger.Error(err, "get serviceaccount failed")
			return ctrl.Result{}, err
		}
	} else {
		if r.Update(ctx, serviceAccount); err != nil {
			logger.Error(err, "update serviceaccount failed")
			return ctrl.Result{}, err
		}
		//logger.Info("Successfully update serviceaccount/" + serviceAccount.GetName())
	}

	// =========================
	// reconcile ClusterRole
	// NOTE: Backup object as namespace-scoped resource doesn't have ability to control/own ClusterRole resource.
	// =========================
	// Construct a clusterrole object.
	clusterRole := r.clusterRoleForBackup(backupObj)
	namespacedName = apitypes.NamespacedName{Namespace: req.NamespacedName.Namespace, Name: "horusctl-role"}
	// get the clusterrole resource.
	if err := r.Get(ctx, namespacedName, &rbacv1.ClusterRole{}); err != nil {
		if apierrors.IsNotFound(err) {
			if err := r.Create(ctx, clusterRole); err != nil {
				logger.Error(err, "create clusterrole failed")
				return ctrl.Result{}, err
			}
			logger.Info("Successfully create clusterrole/" + clusterRole.GetName())
			return ctrl.Result{Requeue: true}, nil
		} else {
			logger.Error(err, "get clusterrole failed")
			return ctrl.Result{}, err
		}
	} else {
		if r.Update(ctx, clusterRole); err != nil {
			logger.Error(err, "update clusterrole failed")
			return ctrl.Result{}, err
		}
		//logger.Info("Successfully update clusterrole/" + clusterRole.GetName())
	}

	// =========================
	// reconcile ClusterRoleBinding
	// NOTE: Backup object as namespace-scoped resource doesn't have ability to control/own ClusterRoleBinding resource.
	// =========================
	// Construct a clusterrolebinding object.
	clusterRoleBinding := r.clusterRoleBindingForBackup(backupObj)
	namespacedName = apitypes.NamespacedName{Namespace: req.NamespacedName.Namespace, Name: fmt.Sprintf("horusctl-%s-binding", req.NamespacedName.Namespace)}
	// get the clusterrolebinding resource.
	if err := r.Get(ctx, namespacedName, &rbacv1.ClusterRoleBinding{}); err != nil {
		if apierrors.IsNotFound(err) {
			if err := r.Create(ctx, clusterRoleBinding); err != nil {
				logger.Error(err, "create clusterrolebinding failed")
				return ctrl.Result{}, err
			}
			logger.Info("Successfully create clusterrolebinding/" + clusterRoleBinding.GetName())
			return ctrl.Result{Requeue: true}, nil
		} else {
			logger.Error(err, "get clusterrolebinding failed")
			return ctrl.Result{}, err
		}
	} else {
		if r.Update(ctx, clusterRoleBinding); err != nil {
			logger.Error(err, "update clusterrolebinding failed")
			return ctrl.Result{}, err
		}
		//logger.Info("Successfully update clusterrolebinding/" + clusterRoleBinding.GetName())
	}

	// =========================
	// reconcile CronJob
	// =========================
	// Construct a cronjob object.
	cronJob := r.cronJobForBackup(ctx, backupObj)
	//r.withNamespace(ctx, cronJob, types.DefaultBackupJobNamespace)
	//namespacedName = apitypes.NamespacedName{Namespace: types.DefaultBackupJobNamespace, Name: "backup" + "-" + req.NamespacedName.Name}
	namespacedName = apitypes.NamespacedName{Namespace: req.NamespacedName.Namespace, Name: "backup" + "-" + req.NamespacedName.Name}
	// get the cronjob resource.
	if err := r.Get(ctx, namespacedName, &batchv1.CronJob{}); err != nil {
		// if cronjob resource not exits, create it.
		if apierrors.IsNotFound(err) {
			if err := r.Create(ctx, cronJob); err != nil {
				logger.Error(err, "create cronjob failed")
				return ctrl.Result{}, err
			}
			logger.Info("Successfully create cronjob/" + cronJob.GetName())
			// cronjob created, return and requeue
			return ctrl.Result{Requeue: true}, nil
		} else {
			// get cronjob error
			logger.Error(err, "get cronjob failed")
			return ctrl.Result{}, err
		}
	} else {
		// if cronjob resource already exist, update it.
		if err := r.Update(ctx, cronJob); err != nil {
			logger.Error(err, "update cronjob failed")
			return ctrl.Result{}, err
		}
		logger.Info("Successfully udpate cronjob/" + cronJob.GetName())
	}

	// NOTE: handler finalizers must be after reconciling ClusterRoleBinding,
	// otherwise ClusterRoleBinding resources will be recreated.
	//
	// Add finalizers when add Backup object
	// Delete finalizers and delete external clusterrolebinding when delete Backup object.
	if err := r.handleFinalizer(ctx, backupObj); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&storagev1alpha1.Backup{}).
		Owns(&batchv1.CronJob{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.ClusterRole{}).
		Owns(&rbacv1.ClusterRoleBinding{}).
		WithEventFilter(predicate.Or(
			common.BackupPredicate(),
			common.ServiceAccountPredicate(),
			common.ClusterRolePredicate(),
			common.ClusterRoleBindingPredicate(),
		)).
		Complete(r)
}

// cronJobForBackup construct a *batch1.CronJob resource that owned/controlled by the Backup resource.
func (r *BackupReconciler) cronJobForBackup(ctx context.Context, backupObj *storagev1alpha1.Backup) *batchv1.CronJob {
	cjData, err := template.Parse(template.CronJobForBackup, backupObj)
	if err != nil {
		r.Log.Error(err, "parse cronjob template failed")
		os.Exit(1)
	}
	cronjob := &batchv1.CronJob{}
	if err := yaml.Unmarshal(cjData, cronjob); err != nil {
		r.Log.Error(err, "unmarshal cronjob failed")
		os.Exit(1)
	}
	ctrl.SetControllerReference(backupObj, cronjob, r.Scheme)
	util.SetRecommendedLabels(cronjob)

	//// The pod generated by this cronjob should have enough permissions to execute Horusctl
	//// command line tool to backup pvc data to storage.
	//// Each cronjob requires serviceaccount and a clusterrolebinding, and the clusterrole
	//// is common one that is bound to multiple serviceaccounts by multiple clusterrolebindings.
	//// It wil stop reconcile if this controller create/apply clusterrole and/or clusterrolebinding
	//// failed during creating the cronjob.
	////
	//// Check ClusterRole, Apply method will create it if not exist, update it if already exists.
	//clusterRole := r.clusterRoleForBackup(backupObj)
	//crHandler := clusterrole.NewOrDie(ctx, "")
	//if _, err := crHandler.Apply(clusterRole); err != nil {
	//    r.Log.Error(err, "clusterrole handler apply clusterrole failed")
	//    os.Exit(1)
	//}
	//// Check ClusterRoleBinding, Apply method will create it if not exist, update it if already exists.
	//clusterRoleBinding := r.clusterRoleBindingForBackup(backupObj)
	//crbHandler := clusterrolebinding.NewOrDie(ctx, "")
	//if _, err := crbHandler.Apply(clusterRoleBinding); err != nil {
	//    // If create clusterrolebinding occur "InValid" error, delete and create it.
	//    // it will occur "InValid" error when update clusterrolebinding.spec.roleRef.
	//    if apierrors.IsInvalid(err) {
	//        crbHandler.Delete(clusterRoleBinding)
	//        if _, err := crbHandler.Create(clusterRoleBinding); err != nil {
	//            r.Log.Error(err, "clusterrolebinding handler create clusterrolebinding failed")
	//            os.Exit(1)
	//        }
	//    }
	//}
	return cronjob
}

// serviceAccountForBackup construct a *corev1.ServiceAccount resource that owned/controlled by the Backup resource.
func (r *BackupReconciler) serviceAccountForBackup(backupObj *storagev1alpha1.Backup) *corev1.ServiceAccount {
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "horusctl",
			Namespace: backupObj.GetNamespace(),
		},
	}
	ctrl.SetControllerReference(backupObj, serviceAccount, r.Scheme)
	util.SetRecommendedLabels(serviceAccount)
	return serviceAccount
}

// clusterRoleForBackup construct a *rbacv1.ServiceAccount resource that owned/controlled by the Backup resource.
func (r *BackupReconciler) clusterRoleForBackup(backupObj *storagev1alpha1.Backup) *rbacv1.ClusterRole {
	crData, err := template.Parse(template.ClusterRoleForBackup, backupObj)
	if err != nil {
		r.Log.Error(err, "parse clusterrole template failed")
		os.Exit(1)
	}
	crObj := &rbacv1.ClusterRole{}
	if err := yaml.Unmarshal(crData, crObj); err != nil {
		r.Log.Error(err, "unmarshal clusterrole failed")
		os.Exit(1)
	}
	// Backup as namespace-scoped resource cannot owns/control cluster-scoped resource.
	ctrl.SetControllerReference(backupObj, crObj, r.Scheme)
	util.SetRecommendedLabels(crObj)
	return crObj
}

// clusterRoleBindingForBackup construct a *rbacv1.ServiceAccount resource that owned/controlled by the Backup resource.
func (r *BackupReconciler) clusterRoleBindingForBackup(backupObj *storagev1alpha1.Backup) *rbacv1.ClusterRoleBinding {
	crbData, err := template.Parse(template.ClusterRoleBindingForBackup, backupObj)
	if err != nil {
		r.Log.Error(err, "parse clusterrolebinding template failed")
		os.Exit(1)
	}
	crbObj := &rbacv1.ClusterRoleBinding{}
	if err := yaml.Unmarshal(crbData, crbObj); err != nil {
		r.Log.Error(err, "unmarshal clusterrolebinding failed")
		os.Exit(1)
	}
	// Backup as namespace-scoped resource cannot owns/control cluster-scoped resource.
	ctrl.SetControllerReference(backupObj, crbObj, r.Scheme)
	util.SetRecommendedLabels(crbObj)
	return crbObj
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
			//logrus.Infof("Add finalizer %s : %t", finalizerName, ok)
			r.Log.Info("Add finalizer", finalizerName, ok)
			return r.Update(ctx, backupObj)
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(backupObj, finalizerName) {
			// our finalizer is present, so lets handle any external dependency
			if err := r.deleteExternalResources(ctx, backupObj); err != nil {
				// if fail to delete the external dependency here, return with error
				// so that it can be retried
				return err
			}
			// remove our finalizer from the list and update it.
			ok := controllerutil.RemoveFinalizer(backupObj, finalizerName)
			//logrus.Infof("Remove finalizer %s : %t", finalizerName, ok)
			r.Log.Info("Remove finalizer", finalizerName, ok)
			return r.Update(ctx, backupObj)
		}
		// Stop reconciliation as the item is being deleted
		return nil
	}
	// Stop reconciliation as the item is being deleted
	return nil
}

// deleteExternalResources
func (r *BackupReconciler) deleteExternalResources(ctx context.Context, backupObj *storagev1alpha1.Backup) error {
	//
	// delete any external resources associated with the cronJob
	//
	// Ensure that delete implementation is idempotent and safe to invoke
	// multiple times for same object.
	crbHandler := clusterrolebinding.NewOrDie(ctx, "")
	crbName := fmt.Sprintf("horusctl-%s-binding", backupObj.GetNamespace())
	// if clusterrolebinding resources not found, Delete method will return "NotFound" error,
	// we should ignore the "NotFound" eror.
	//if err := crbHandler.Delete(crbName); k8serrors.IgnoreNotFound(err) != nil {
	if err := crbHandler.Delete(crbName); err != nil {
		return errors.Wrapf(err, "clusterrolebinding handler delete clusterrolebinding/%s failed", crbName)
	}
	return nil
}

// withNamespace set the object namespace to the provided namespace.
// If the provided namespace is not same to the object original namespace,
// it will remove .metadata.ownerReferences field.
// If the namespace not exists, it will create it.
func (r *BackupReconciler) withNamespace(ctx context.Context, object client.Object, name string) client.Object {
	handler := namespace.NewOrDie(ctx, "")
	nsObj := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	if _, err := handler.Apply(nsObj); err != nil {
		r.Log.Error(err, "namespace handler apply namespace failed")
		os.Exit(1)
	}

	originalNamespace := object.GetNamespace()
	if name != originalNamespace {
		object.SetNamespace(name)
		object.SetOwnerReferences(nil)
	}
	return object
}
