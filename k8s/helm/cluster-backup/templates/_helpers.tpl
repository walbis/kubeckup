{{/*
Expand the name of the chart.
*/}}
{{- define "cluster-backup.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "cluster-backup.fullname" -}}
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
{{- define "cluster-backup.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "cluster-backup.labels" -}}
helm.sh/chart: {{ include "cluster-backup.chart" . }}
{{ include "cluster-backup.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- with .Values.extra.labels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "cluster-backup.selectorLabels" -}}
app.kubernetes.io/name: {{ include "cluster-backup.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use for backup
*/}}
{{- define "cluster-backup.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (printf "%s-backup" (include "cluster-backup.fullname" .)) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the service account to use for git-sync
*/}}
{{- define "cluster-backup.gitSyncServiceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- printf "%s-git-sync" (include "cluster-backup.fullname" .) }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create namespace name
*/}}
{{- define "cluster-backup.namespace" -}}
{{- default .Release.Namespace .Values.global.namespaceOverride }}
{{- end }}

{{/*
Create backup image name
*/}}
{{- define "cluster-backup.backupImage" -}}
{{- $registry := .Values.global.imageRegistry | default .Values.image.backup.registry }}
{{- printf "%s/%s:%s" $registry .Values.image.backup.repository (.Values.image.backup.tag | default .Chart.AppVersion) }}
{{- end }}

{{/*
Create git-sync image name
*/}}
{{- define "cluster-backup.gitSyncImage" -}}
{{- $registry := .Values.global.imageRegistry | default .Values.image.gitSync.registry }}
{{- printf "%s/%s:%s" $registry .Values.image.gitSync.repository (.Values.image.gitSync.tag | default .Chart.AppVersion) }}
{{- end }}

{{/*
Create MinIO endpoint
*/}}
{{- define "cluster-backup.minioEndpoint" -}}
{{- .Values.minio.endpoint }}
{{- end }}

{{/*
Create pull policy
*/}}
{{- define "cluster-backup.pullPolicy" -}}
{{- if .Values.development.enabled }}
{{- .Values.development.imagePullPolicy | default .Values.image.backup.pullPolicy }}
{{- else }}
{{- .Values.image.backup.pullPolicy }}
{{- end }}
{{- end }}

{{/*
Common annotations
*/}}
{{- define "cluster-backup.annotations" -}}
{{- with .Values.extra.annotations }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
Pod security context
*/}}
{{- define "cluster-backup.podSecurityContext" -}}
{{- if not .Values.development.enabled }}
{{- toYaml .Values.security.podSecurityContext }}
{{- end }}
{{- end }}

{{/*
Container security context
*/}}
{{- define "cluster-backup.securityContext" -}}
{{- if not .Values.development.enabled }}
{{- toYaml .Values.security.securityContext }}
{{- end }}
{{- end }}