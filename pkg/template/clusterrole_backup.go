package template

var (
	// The ClusterRole for horusctl to backup/restore/clone/migration pvc data.
	ClusterRoleForBackup = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: horusctl-role
rules:
# permissions for horusctl to view backups,restores,clones,migrations,traffics.
- apiGroups:
  - storage.hybfkuf.io
  - networking.hybfkuf.io
  resources:
  - backups
  - restores
  - clones
  - migrations
  - traffics
  verbs:
  - get
  - list
  - watch
# permissions for horusctl to view status subresources of backups,restores,clones,migrations,traffics.
- apiGroups:
  - storage.hybfkuf.io
  - networking.hybfkuf.io
  resources:
  - backups/status
  - restores/status
  - clones/status
  - migrations/status
  - traffics/status
  verbs:
  - get
- apiGroups:
  - rbac.authorization.k8s.io
  resources:
  - clusterrolebindings
  - clusterroles
  verbs:
  - create
  - delete
  - get
  - list
  - patch
  - update
  - watch
# permissions for horusctl to create/update/delete deployments,
# and for horusctl-operator-controller-manager to create namespaces.
- apiGroups:
  - ""
  - apps
  resources:
  - deployments
  - namespaces
  verbs:
  - get
  - list
  - watch
  - create
  - delete
  - update
  - patch
# permissions for horusctl to view pods,deployments,statefulsets,daemonsets,replicasets,
# secrets,persistentvolumes, persistentvolumeclaims.
- apiGroups:
  - ""
  - apps
  resources:
  - pods
  - deployments
  - statefulsets
  - daemonsets
  - replicasets
  - secrets
  - persistentvolumes
  - persistentvolumeclaims
  verbs:
  - get
  - list
  - watch
# permissions for horusctl to execute command within pod.
- apiGroups:
  - ""
  resources:
  - pods/exec
  verbs:
  - get
  - create
# permissions for horusctl to get pod logs.
- apiGroups:
  - ""
  resources:
  - pods/logs
  verbs:
  - get
`
)
