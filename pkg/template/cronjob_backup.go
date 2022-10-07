package template

var (
	CronJobForBackup = `
apiVersion: batch/v1
kind: CronJob
metadata:
  name: backup-{{.ObjectMeta.Name}}
  namespace: default
spec:
  schedule: '*/1 * * * *'
  successfulJobsHistoryLimit: 1
  failedJobsHistoryLimit: 1
  concurrencyPolicy: Forbid
  suspend: false
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - command:
            - horusctl
            args:
            - --log-level={{.Spec.LogLevel}}
            - --log-format={{.Spec.LogFormat}}
            - backup
            - --namespace={{.ObjectMeta.Namespace}}
            - {{.Spec.BackupFrom.Name}}
            env:
            - name: TZ
              value: {{.Spec.TimeZone}}
            image: hybfkuf/horusctl:latest
            imagePullPolicy: Always
            name: horusctl
          restartPolicy: Never
          serviceAccount: horusctl
          serviceAccountName: horusctl
`
)
