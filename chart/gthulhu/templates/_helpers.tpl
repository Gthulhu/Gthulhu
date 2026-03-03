{{/*
Expand the name of the chart.
*/}}
{{- define "gthulhu.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "gthulhu.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "gthulhu.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "gthulhu.labels" -}}
helm.sh/chart: {{ include "gthulhu.chart" . }}
{{ include "gthulhu.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "gthulhu.selectorLabels" -}}
app.kubernetes.io/name: {{ include "gthulhu.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "gthulhu.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "gthulhu.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
MongoDB host for connection string (referencing subchart)
*/}}
{{- define "gthulhu.mongodbHost" -}}
{{ .Release.Name }}-mongodb-0.{{ .Release.Name }}-mongodb.{{ .Release.Namespace }}.svc.cluster.local
{{- end }}

{{/*
MongoDB auth secret name (referencing subchart)
*/}}
{{- define "gthulhu.mongodbAuthSecretName" -}}
{{- if .Values.mongodb.auth.existingSecret }}
{{- .Values.mongodb.auth.existingSecret }}
{{- else }}
{{- printf "%s-mongodb-auth" .Release.Name }}
{{- end }}
{{- end }}

{{/*
mTLS Secret name — returns existingSecret if set, otherwise the chart-managed name.
*/}}
{{- define "gthulhu.mtlsSecretName" -}}
{{- if .Values.mtls.existingSecret }}
{{- .Values.mtls.existingSecret }}
{{- else }}
{{- printf "%s-mtls-certs" (include "gthulhu.fullname" .) }}
{{- end }}
{{- end }}
