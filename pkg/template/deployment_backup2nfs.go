package template

var (
	Backup2nfsDeploymentTemplate = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: "%s"
  namespace: "%s"
  labels:
    app.kubernetes.io/name: backup-to-nfs
    app.kubernetes.io/part-of: horus-operator
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: backup-to-nfs
      app.kubernetes.io/part-of: horus-operator
  template:
    metadata:
      annotations:
      #  %s: %s
        sidecar.istio.io/inject: "false"
      labels:
        app.kubernetes.io/name: backup-to-nfs
        app.kubernetes.io/role: backup
        app.kubernetes.io/backup-method: restic
        app.kubernetes.io/part-of: horus-operator
    spec:
      nodeName: "%s"
      tolerations:
      - operator: Exists
      containers:
      - name: backup-to-nfs
        image: "%s"
        env:
        - name: TZ
          value: %s
        - name: RESTIC_REPOSITORY
          value: %s
        - name: RESTIC_PASSWORD
          valueFrom:
            secretKeyRef:
              name: %s
              key: RESTIC_PASSWORD
        volumeMounts:
        - name: host-root
          mountPath: /host-root
          readOnly: true
        - name: restic-repo
          mountPath: "%s"
          readOnly: false
      volumes:
      - name: host-root
        hostPath:
          path: /
          type: Directory
      - name: restic-repo
        nfs:
          server: "%s"
          path: "%s"
`
)
