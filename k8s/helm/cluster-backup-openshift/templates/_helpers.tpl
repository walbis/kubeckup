{{/*
Expand the name of the chart.
*/}}
{{- define "cluster-backup-openshift.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "cluster-backup-openshift.fullname" -}}
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
{{- define "cluster-backup-openshift.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "cluster-backup-openshift.labels" -}}
helm.sh/chart: {{ include "cluster-backup-openshift.chart" . }}
{{ include "cluster-backup-openshift.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
platform: openshift
{{- end }}

{{/*
Selector labels
*/}}
{{- define "cluster-backup-openshift.selectorLabels" -}}
app.kubernetes.io/name: {{ include "cluster-backup-openshift.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use for backup
*/}}
{{- define "cluster-backup-openshift.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (printf "%s-backup" (include "cluster-backup-openshift.fullname" .)) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the backup ConfigMap
*/}}
{{- define "cluster-backup-openshift.configMapName" -}}
{{- printf "%s-config" (include "cluster-backup-openshift.fullname" .) }}
{{- end }}

{{/*
Create the name of the backup Secret
*/}}
{{- define "cluster-backup-openshift.secretName" -}}
{{- printf "%s-secrets" (include "cluster-backup-openshift.fullname" .) }}
{{- end }}

{{/*
Create backup image name
*/}}
{{- define "cluster-backup-openshift.backupImage" -}}
{{- $registry := .Values.global.imageRegistry | default .Values.image.backup.registry }}
{{- if $registry }}
{{- printf "%s/%s:%s" $registry .Values.image.backup.repository .Values.image.backup.tag }}
{{- else }}
{{- printf "%s:%s" .Values.image.backup.repository .Values.image.backup.tag }}
{{- end }}
{{- end }}

{{/*
Create git-sync image name
*/}}
{{- define "cluster-backup-openshift.gitSyncImage" -}}
{{- $registry := .Values.global.imageRegistry | default .Values.image.gitSync.registry }}
{{- if $registry }}
{{- printf "%s/%s:%s" $registry .Values.image.gitSync.repository .Values.image.gitSync.tag }}
{{- else }}
{{- printf "%s:%s" .Values.image.gitSync.repository .Values.image.gitSync.tag }}
{{- end }}
{{- end }}

{{/*
Create namespace name
*/}}
{{- define "cluster-backup-openshift.namespace" -}}
{{- default .Release.Namespace .Values.global.namespaceOverride }}
{{- end }}

{{/*
Create cluster name
*/}}
{{- define "cluster-backup-openshift.clusterName" -}}
{{- .Values.cluster.name }}
{{- end }}

{{/*
Common backup component labels
*/}}
{{- define "cluster-backup-openshift.backupLabels" -}}
{{ include "cluster-backup-openshift.labels" . }}
app.kubernetes.io/component: backup
{{- end }}

{{/*
Common git-sync component labels
*/}}
{{- define "cluster-backup-openshift.gitSyncLabels" -}}
{{ include "cluster-backup-openshift.labels" . }}
app.kubernetes.io/component: git-sync
{{- end }}

{{/*
Backup selector labels
*/}}
{{- define "cluster-backup-openshift.backupSelectorLabels" -}}
{{ include "cluster-backup-openshift.selectorLabels" . }}
app.kubernetes.io/component: backup
{{- end }}

{{/*
Git-sync selector labels
*/}}
{{- define "cluster-backup-openshift.gitSyncSelectorLabels" -}}
{{ include "cluster-backup-openshift.selectorLabels" . }}
app.kubernetes.io/component: git-sync
{{- end }}