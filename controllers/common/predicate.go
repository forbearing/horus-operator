package common

import (
	"github.com/forbearing/horus-operator/pkg/types"
	"github.com/forbearing/k8s/util/labels"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// BackupPredicate.
func BackupPredicate() predicate.Predicate {
	return predicate.Funcs{
		// Ignore updates to Backup object status in which case  metadata.Generation does not change
		UpdateFunc: func(e event.UpdateEvent) bool { return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration() },
		// Evaluates to false if the object has confirmed deleted.
		DeleteFunc: func(e event.DeleteEvent) bool {
			return !e.DeleteStateUnknown
		},
	}
}

// RestorePredicate
func RestorePredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool { return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration() },
		DeleteFunc: func(e event.DeleteEvent) bool { return !e.DeleteStateUnknown },
	}
}

// ClonePredicate
func ClonePredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool { return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration() },
		DeleteFunc: func(e event.DeleteEvent) bool { return !e.DeleteStateUnknown },
	}
}

// MigrationPredicate
func MigrationPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool { return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration() },
		DeleteFunc: func(e event.DeleteEvent) bool { return !e.DeleteStateUnknown },
	}
}

// TrafficPredicate
func TrafficPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool { return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration() },
		DeleteFunc: func(e event.DeleteEvent) bool { return !e.DeleteStateUnknown },
	}
}

// ServiceAccountPredicate
func ServiceAccountPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool { return labels.Has(e.Object, types.LabelPairManagedBy) },
		UpdateFunc: func(e event.UpdateEvent) bool { return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration() },
		DeleteFunc: func(e event.DeleteEvent) bool { return !e.DeleteStateUnknown },
	}
}

// ClusterRolePredicate
// Backup as namespace-scoped resource cannot owns/control cluster-scoped resource.
// When ClusterRole was deleted, horus-operator-controller-manager couldn't recreate it again.
// We should reconcile agin to recreate the ClusterRole when we watch ClusterRole deleted event.
func ClusterRolePredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool { return labels.Has(e.Object, types.LabelPairManagedBy) },
		UpdateFunc: func(e event.UpdateEvent) bool { return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration() },
		//DeleteFunc: func(e event.DeleteEvent) bool { return !e.DeleteStateUnknown },
	}
}

// ClusterRoleBindingPredicate
// Backup as namespace-scoped resource cannot owns/control cluster-scoped resource.
// When ClusterRoleBinding was deleted, horus-operator-controller-manager couldn't recreate it again.
// We should reconcile agin to recreate the ClusterRoleBinding when we watch ClusterRole deleted event.
func ClusterRoleBindingPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool { return labels.Has(e.Object, types.LabelPairManagedBy) },
		UpdateFunc: func(e event.UpdateEvent) bool { return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration() },
		//DeleteFunc: func(e event.DeleteEvent) bool { return !e.DeleteStateUnknown },
	}
}
