package template

var (
	HorusctlImage = "hybfkuf/horusctl:latest"

	HorusctlCronjobTemplate = `
#apiVersion: batch/v1
apiVersion: batch/v1beta1
kind: CronJob
metadata:
  name: "%s"
  namespace: "%s"
  labels:
    app.kubernetes.io/name: "horusctl"
    app.kubernetes.io/role: backup
    app.kubernetes.io/backup-tool: restic
    app.kubernetes.io/part-of: horus-operator
spec:
  schedule: "%s"
  concurrencyPolicy: Forbid
  successfulJobsHistoryLimit: 3
  failedJobsHistoryLimit: 3
  jobTemplate:
    spec:
      backoffLimit: 2
      template:
        metadata:
          labels:
            app.kubernetes.io/name: "horusctl"
            app.kubernetes.io/role: backup
            app.kubernetes.io/backup-tool: restic
            app.kubernetes.io/part-of: horus-operator
          annotations:
            sidecar.istio.io/inject: "false"
        spec:
          restartPolicy: Never
          containers:
          - name: horusctl
            image: "%s"
            imagePullPolicy: Always
            command:
            - horusctl
            - backup
            - --namespace %s 
            - '%s'
            env:
            - name: TZ
              value: "%s"
            securityContext:
              privileged: true
`
)
