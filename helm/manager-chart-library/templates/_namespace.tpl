{{- define "manager-chart-library.namespace" -}}
apiVersion: v1
kind: Namespace
metadata:
  labels:
    control-plane: {{ .Values.namespace.label }}
  name: {{ .Values.namespace.name }}

{{- end -}}