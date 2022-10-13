package template

var (
	TemplateBackup2minio = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: "%s"
  namespace: "%s"
  labels:
    app.kubernetes.io/name: backup-to-minio
    app.kubernetes.io/part-of: horus-operator
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: backup-to-minio
      app.kubernetes.io/part-of: horus-operator
  template:
    metadata:
      annotations:
      #  %s: %s
        sidecar.istio.io/inject: "false"
      labels:
        app.kubernetes.io/name: backup-to-minio
        app.kubernetes.io/role: backup
        app.kubernetes.io/backup-method: restic
        app.kubernetes.io/part-of: horus-operator
       
    spec:
      nodeName: "%s"
      tolerations:
      - operator: Exists
      terminationGracePeriodSeconds: 0
      containers:
      - name: backup-to-minio
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
        - name: MINIO_ACCESS_KEY
          valueFrom:
            secretKeyRef:
              name: %s
              key: MINIO_ACCESS_KEY
        - name: MINIO_SECRET_KEY
          valueFrom:
            secretKeyRef:
              name: %s
              key: MINIO_SECRET_KEY
        - name: AWS_ACCESS_KEY_ID
          valueFrom:
            secretKeyRef:
              name: %s
              key: MINIO_ACCESS_KEY
        - name: AWS_SECRET_ACCESS_KEY
          valueFrom:
            secretKeyRef:
              name: %s
              key: MINIO_SECRET_KEY
        volumeMounts:
        - name: host-root
          mountPath: /host-root
          readOnly: true
      volumes:
      - name: host-root
        hostPath:
          path: /
          type: Directory
`
)
