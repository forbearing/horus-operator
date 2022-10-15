package template

var (
	TemplateBackup2sftp = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: "%s"
  namespace: "%s"
  labels:
    app.kubernetes.io/name: backup-to-sftp
    app.kubernetes.io/part-of: horus
    app.kubernetes.io/managed-by: horus-operator
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: backup-to-sftp
      app.kubernetes.io/part-of: horus
      app.kubernetes.io/managed-by: horus-operator
  template:
    metadata:
      annotations:
      #  %s: %s
        sidecar.istio.io/inject: "false"
      labels:
        app.kubernetes.io/name: backup-to-sftp
        app.kubernetes.io/role: backup
        app.kubernetes.io/backup-method: restic
        app.kubernetes.io/part-of: horus
        app.kubernetes.io/managed-by: horus-operator
    spec:
      nodeName: "%s"
      tolerations:
      - operator: Exists
      terminationGracePeriodSeconds: 0
      containers:
      - name: backup-to-sftp
        image: "%s"
        env:
        - name: TZ
          value: %s
        - name: STORAGE
          value: %s
        - name: RESTIC_REPOSITORY
          value: %s
        - name: RESTIC_PASSWORD
          valueFrom:
            secretKeyRef:
              name: %s
              key: RESTIC_PASSWORD
        - name: SFTP_USERNAME
          valueFrom:
            secretKeyRef:
              name: %s
              key: SFTP_USERNAME
        - name: SFTP_PASSWORD
          valueFrom:
            secretKeyRef:
              name: %s
              key: SFTP_PASSWORD
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
