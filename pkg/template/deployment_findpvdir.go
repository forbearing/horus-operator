package template

var (
	TemplateFindpvdir = `
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
      #annotations:
      #  %s: %s
      labels:
        app.kubernetes.io/name: findpvdir
        app.kubernetes.io/part-of: horus-operator
    spec:
      nodeName: %s
      tolerations:
      - operator: Exists
      terminationGracePeriodSeconds: 0
      containers:
      - name: findpvdir
        image: %s
        env:
        - name: TZ
          value: %s
        resources:
          limits:
            cpu: 0.1
            memory: 50Mi
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
)
