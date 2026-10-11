{{- define "kithly.labels" -}}
app.kubernetes.io/part-of: kithly
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
{{- end -}}

{{/* Selector labels for one component; args: (dict "root" $ "name" <component>) */}}
{{- define "kithly.selectorLabels" -}}
app.kubernetes.io/name: {{ .name }}
app.kubernetes.io/instance: {{ .root.Release.Name }}
{{- end -}}

{{- define "kithly.redisHost" -}}
{{ .Release.Name }}-redis
{{- end -}}

{{/*
pg_dump container for the backup CronJob; args: (dict "root" $ "name" <postgres name> "backup" <backup values>).
Writes a custom-format dump to the backups PVC, records its path in LATEST, and prunes old dumps.
*/}}
{{- define "kithly.postgresDumpContainer" -}}
- name: dump
  image: {{ .root.Values.postgres.image }}
  command: ["sh", "-ec"]
  args:
    - |
      f="/backups/{{ .root.Values.postgres.database }}-$(date -u +%Y%m%dT%H%M%SZ).dump"
      pg_dump --format=custom --no-owner --file="$f.partial"
      mv "$f.partial" "$f"
      echo "$f" > /backups/LATEST
      find /backups -name '*.dump' -mtime +{{ .backup.retentionDays }} -print -delete
      echo "backup written: $f ($(du -h "$f" | cut -f1))"
  env:
    - name: PGHOST
      value: {{ .name }}
    - name: PGDATABASE
      value: {{ .root.Values.postgres.database | quote }}
    - name: PGUSER
      value: {{ .root.Values.postgres.user | quote }}
    - name: PGPASSWORD
      valueFrom:
        secretKeyRef:
          name: {{ .name }}
          key: POSTGRES_PASSWORD
  volumeMounts:
    - name: backups
      mountPath: /backups
{{- end -}}
