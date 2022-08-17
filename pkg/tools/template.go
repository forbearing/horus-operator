package tools

var (
	findpvpathDeployTemplate = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: %s
  labels:
    app.kubernetes.io/name: findpvpath
    app.kubernetes.io/part-of: horus-operator
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: findpvpath
      app.kubernetes.io/part-of: horus-operator
  template:
    metadata:
      labels:
        app.kubernetes.io/name: findpvpath
        app.kubernetes.io/part-of: horus-operator
    spec:
      nodeName: %s
      containers:
      - name: findpvpath
        image: hybfkuf/findpvpath:latest
        volumeMounts:
        - name: kubelet-home-dir
          mountPath: /var/lib/kubelet
      volumes:
      - name: kubelet-home-dir
        hostPath:
          path: /var/lib/kubelet
          type: Directory
`
)

var findpvpathPod = `
apiVersion: v1
kind: Pod
metadata:
  name: %s
  namespace: %s
spec:
  nodeName: %s
  containers:
  - name: findpvpath
    image: hybfkuf/findpvpath:latest
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

var podTemplateForNFS = `
apiVersion: v1
kind: Pod
metadata:
  name: %s
  namespace: %s
spec:
  nodeName: %s
  containers:
  - name: backup
  image: hybfkuf/backup-tools-restic:latest
  volumes:
  - name: backup-from
    hostPath:
      type: Directory
	  path: %s
  - name: backup-to
    nfs:
      server: %s
      path: %s
`
