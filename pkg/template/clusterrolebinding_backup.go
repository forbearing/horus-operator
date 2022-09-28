package template

var (
	ClusterRoleBindingForBackup = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: horusctl-{{.ObjectMeta.Namespace}}-binding
subjects:
- kind: ServiceAccount
  name: horusctl
  namespace: {{.ObjectMeta.Namespace}}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: horusctl-role
`
)
