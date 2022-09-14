package tools

var (
	findpvdirDeploymentTemplate = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: %s
  labels:
    app.kubernetes.io/name: findpvdir
    app.kubernetes.io/part-of: horus-operator
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: findpvdir
      app.kubernetes.io/part-of: horus-operator
  template:
    metadata:
      annotations:
        %s: %s
      labels:
        app.kubernetes.io/name: findpvdir
        app.kubernetes.io/part-of: horus-operator
    spec:
      nodeName: %s
      containers:
      - name: findpvdir
        image: %s
        volumeMounts:
        - name: kubelet-home-dir
          mountPath: /var/lib/kubelet
          readOnly: true
      volumes:
      - name: kubelet-home-dir
        hostPath:
          path: /var/lib/kubelet
          type: Directory
`

	backuptonfsDeploymentTemplate = `
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
        %s: %s
      labels:
        app.kubernetes.io/name: backup-to-nfs
        app.kubernetes.io/part-of: horus-operator
    spec:
      nodeName: "%s"
      containers:
      - name: backup-to-nfs
        image: "%s"
        volumeMounts:
        - name: backup-source
          mountPath: "%s"
          readOnly: true
        - name: restic-repo
          mountPath: restic-repo
          readOnly: false
      volumes:
      - name: "backup-source"
        hostPath:
          path: "%s"
          type: Directory
      - name: restic-repo
        nfs:
          server: "%s"
          path: "%s"
`
)
